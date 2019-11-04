package jsonschema

import (
	"fmt"

	"path/filepath"

	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/e2e/jsonschema/schema"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
)

// EventTypeToSchemaFilename maps event types with their json schema.
type EventTypeToSchemaFilename map[string]string

// ErrMatch is the error that Match function returns
type ErrMatch struct {
	errs []error
}

func (errMatch ErrMatch) Error() string {
	var out string
	for _, e := range errMatch.errs {
		out = fmt.Sprintf("\n%s\t- %s\n", out, e)
	}

	return out
}

// MatchIntegration matches an integration against a JSON schema defined for an
// Infrastructure Integration.
func MatchIntegration(o *sdk.IntegrationProtocol2) error {
	return validate(gojsonschema.NewStringLoader(schema.IntegrationSchema), gojsonschema.NewGoLoader(o))
}

// MatchEntities matches metric sets of entities against a set of JSON schema
// for each event type.
func MatchEntities(data []*sdk.EntityData, schemaFileByJobByType map[string]EventTypeToSchemaFilename, schemasDir string) error {
	var errs []error
	missingSchemas := make(map[string]struct{})
	foundTypes := make(map[string]struct{})

	expectedEvents := make(map[string]string)
	for jobName, eventTypeToSchema := range schemaFileByJobByType {
		for event := range eventTypeToSchema {
			expectedEvents[event] = jobName
		}
	}

	for _, entityData := range data {
		for _, metric := range entityData.Metrics {
			eventType := metric["event_type"].(string)
			_, found := expectedEvents[eventType]
			if !found {
				missingSchemas[eventType] = struct{}{}
				continue
			}

			foundTypes[eventType] = struct{}{}
			job := expectedEvents[eventType]
			schemaFilename := schemaFileByJobByType[job][eventType]
			fp, err := schemaFilepath(schemaFilename, schemasDir)
			if err != nil {
				errs = append(errs, fmt.Errorf("found event %s, but schema not found", eventType))
				continue
			}

			err = validate(gojsonschema.NewReferenceLoader(fp), gojsonschema.NewGoLoader(metric))
			if err != nil {
				entity := entityData.Entity
				errMsg := fmt.Errorf("%s:%s %s:\n%s", entity.Type, entity.Name, metric["event_type"], err)
				errs = append(errs, errMsg)
			}
		}
	}

	var terr string
	for t := range expectedEvents {
		if _, ok := foundTypes[t]; !ok {
			terr = fmt.Sprintf("%s%s, ", terr, t)
		}
	}
	if len(terr) > 0 {
		errs = append(errs, fmt.Errorf("mandatory types were not found: %s", terr))
	}

	if len(missingSchemas) > 0 {
		e := fmt.Sprint("some types were not validated because no schema was found: ")
		for t := range missingSchemas {
			e = fmt.Sprintf("%s%s, ", e, t)
		}

		errs = append(errs, errors.New(e))
	}

	if len(errs) > 0 {
		return ErrMatch{errs: errs}
	}

	return nil
}

func schemaFilepath(filename string, dir string) (string, error) {
	schemas := filepath.Join(dir, filename)
	abs, err := filepath.Abs(schemas)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("file://%s", abs), nil
}

func validate(schemaLoader, documentLoader gojsonschema.JSONLoader) error {
	r, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if r.Valid() {
		return nil
	}

	var validationErrsMsg string
	for _, desc := range r.Errors() {
		validationErrsMsg = fmt.Sprintf("%s\t\t- %s\n", validationErrsMsg, desc)
	}

	return errors.New(validationErrsMsg)
}
