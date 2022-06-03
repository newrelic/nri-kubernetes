package definition

import (
	"github.com/newrelic/infra-integrations-sdk/data/metric"
)

// EntityIDGeneratorFunc generates an entity ID.
type EntityIDGeneratorFunc func(groupLabel, rawEntityID string, g RawGroups) (string, error)

// EntityTypeGeneratorFunc generates an entity type.
type EntityTypeGeneratorFunc func(groupLabel, rawEntityID string, g RawGroups, prefix string) (string, error)

// NamespaceGetterFunc gets the namepsace.
type NamespaceGetterFunc func(metrics RawMetrics) string

// Spec is a metric specification.
type Spec struct {
	Name      string
	ValueFunc FetchFunc
	Type      metric.SourceType
	Optional  bool
}

// SpecGroup represents a bunch of specs that share logic.
type SpecGroup struct {
	IDGenerator     EntityIDGeneratorFunc
	TypeGenerator   EntityTypeGeneratorFunc
	NamespaceGetter NamespaceGetterFunc
	Specs           []Spec
}

// SpecGroups is a map of groups indexed by group name.
type SpecGroups map[string]SpecGroup
