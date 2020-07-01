package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/cache"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/pkg/errors"
)

// Entity is the producer of the data. Entity could be a host, a container, a pod, or whatever unit of meaning.
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// EntityData defines all the data related to a particular event from an entity.
type EntityData struct {
	Entity    Entity             `json:"entity"`
	Metrics   []metric.MetricSet `json:"metrics"`
	Inventory Inventory          `json:"inventory"`
	Events    []*Event           `json:"events"`
}

// NewEntityData creates a new EntityData with default values initialised.
func NewEntityData(entityName, entityType string) (EntityData, error) {
	// If one of the attributes is defined, both Name and Type are needed.
	if entityName == "" && entityType != "" || entityName != "" && entityType == "" {
		return EntityData{}, errors.New("entity name and type are required when defining one")
	}

	d := EntityData{
		// empty array or object preferred instead of null on marshaling.
		Metrics:   []metric.MetricSet{},
		Inventory: Inventory{},
		Events:    []*Event{},
	}

	// Entity data is optional. When not specified, data from the integration is reported for the agent's own entity.
	if entityName != "" && entityType != "" {
		d.Entity = Entity{
			Name: entityName,
			Type: entityType,
		}
	}

	return d, nil
}

// IntegrationProtocol2 defines the format of the output JSON that integrations will return for protocol 2.
type IntegrationProtocol2 struct {
	sync.Mutex
	Name               string        `json:"name"`
	ProtocolVersion    string        `json:"protocol_version"`
	IntegrationVersion string        `json:"integration_version"`
	Data               []*EntityData `json:"data"`
	prettyOutput       bool
	writer             io.Writer
}

// NewIntegrationProtocol2WithWriter initializes a new instance of the integration protocol 2 specifying a writer instead of using stdout by default.
func NewIntegrationProtocol2WithWriter(name string, version string, arguments interface{}, writer io.Writer) (*IntegrationProtocol2, error) {
	i, err := NewIntegrationProtocol2(name, version, arguments)
	if err != nil {
		return nil, err
	}

	i.writer = writer

	return i, nil
}

// NewIntegrationProtocol2 initializes a new instance of the integration protocol 2.
func NewIntegrationProtocol2(name string, version string, arguments interface{}) (*IntegrationProtocol2, error) {
	err := args.SetupArgs(arguments)
	if err != nil {
		return nil, err
	}
	defaultArgs := args.GetDefaultArgs(arguments)

	log.SetupLogging(defaultArgs.Verbose)

	// Avoid working with an uninitialized or in error state cache
	if err = cache.Status(); err != nil {
		return nil, err
	}

	integration := &IntegrationProtocol2{
		Name:               name,
		ProtocolVersion:    "2",
		IntegrationVersion: version,
		prettyOutput:       defaultArgs.Pretty,
		Data:               []*EntityData{}, // we do because we don't want a null but an empty array on marshaling.
		writer:             os.Stdout,       // StdOut by default
	}

	return integration, nil
}

// Entity method creates or retrieves an already created EntityData.
func (integration *IntegrationProtocol2) Entity(entityName, entityType string) (*EntityData, error) {
	integration.Lock()
	defer integration.Unlock()
	for _, e := range integration.Data {
		if e.Entity.Name == entityName && e.Entity.Type == entityType {
			return e, nil
		}
	}

	d, err := NewEntityData(entityName, entityType)
	if err != nil {
		return nil, err
	}

	integration.Data = append(integration.Data, &d)

	return &d, nil
}

// NewMetricSet returns a new instance of MetricSet with its sample attached to
// the IntegrationData.
func (d *EntityData) NewMetricSet(eventType string) metric.MetricSet {
	ms := metric.NewMetricSet(eventType)
	if d.Entity.Type != "" && d.Entity.Name != "" {
		ms.SetMetric("entityName", fmt.Sprintf("%s:%s", d.Entity.Type, d.Entity.Name), metric.ATTRIBUTE)
	}
	d.Metrics = append(d.Metrics, ms)

	return ms
}

// AddNotificationEvent method adds a new Event with default event category.
func (d *EntityData) AddNotificationEvent(summary string) error {
	return d.AddEvent(Event{Summary: summary, Category: DefaultEventCategory})
}

// AddEvent method adds a new Event.
func (d *EntityData) AddEvent(e Event) error {
	if e.Summary == "" {
		return fmt.Errorf("summary of the event cannot be empty")
	}

	d.Events = append(d.Events, &e)
	return nil
}

// Publish runs all necessary tasks before publishing the data. Currently, it
// stores the cache, prints the JSON representation of the integration using a writer (stdout by default)
// and re-initializes the integration object (allowing re-use it during the
// execution of your code).
func (integration *IntegrationProtocol2) Publish() error {
	if err := cache.Save(); err != nil {
		return err
	}

	output, err := integration.toJSON(integration.prettyOutput)
	if err != nil {
		return err
	}

	fmt.Fprint(integration.writer, output)
	integration.Clear()

	return nil
}

// Clear re-initializes the Inventory, Metrics and Events for this integration.
// Used after publishing so the object can be reused.
func (integration *IntegrationProtocol2) Clear() {
	integration.Lock()
	defer integration.Unlock()
	integration.Data = []*EntityData{} // empty array preferred instead of null on marshaling.
}

// toJSON returns the integration as a JSON string. If the pretty attribute is
// set to true, the JSON will be idented for easy reading.
func (integration *IntegrationProtocol2) toJSON(pretty bool) (string, error) {
	var output []byte
	var err error

	if pretty {
		output, err = json.MarshalIndent(integration, "", "\t")
	} else {
		output, err = json.Marshal(integration)
	}

	if err != nil {
		return "", fmt.Errorf("error marshalling to JSON: %s", err)
	}

	if string(output) == "null" {
		return "[]", nil
	}

	return string(output), nil
}
