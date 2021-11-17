package testutil

import (
	"strings"
	"sync"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

// Asserter is a helper for checking whether an integration contains all the metrics defined in a specGroup.
// It provides a chainable API, with each call returning a copy of the asserter. This way, successive calls to the
// chainable methods do not modify the previous Asserter, allowing to reuse the chain as a test fans out.
// Asserter is safe to use concurrently.
type Asserter struct {
	*sync.Mutex
	entities        []*integration.Entity
	specGroups      definition.SpecGroups
	exclude         exclusions
	excludeOptional bool
	silent          bool
}

// exclusions is a map from groupName to metricName to a boolean which is true if the metric should be excluded.
// An entry for a groupName with an empty list of metrics to exclude will be interpreted as an exclusion for all metrics
// in that group.
type exclusions map[string]map[string]bool

func NewAsserter() Asserter {
	return Asserter{
		Mutex: &sync.Mutex{},
	}
}

// Using returns an asserter that will use the supplied specGroups to assert entities.
func (a Asserter) Using(groups definition.SpecGroups) Asserter {
	a.specGroups = groups
	return a
}

// On returns an asserter configured to check for existence on the supplied entities.
func (a Asserter) On(entities []*integration.Entity) Asserter {
	a.entities = entities
	return a
}

// Excluding returns an asserter that will not fail for the supplied groupName and metricName if they are missing.
// If no metricNames are specified, Asserter will ignore the whole group.
// Missing metrics are still logged.
func (a Asserter) Excluding(groupName string, metricNames ...string) Asserter {
	// Map is a pointer and therefore shared among all asserters. To avoid modifying the previous asserter we need to
	// copy it.
	prevExclude := a.exclude
	a.exclude = map[string]map[string]bool{}

	// Copy exclusions from previous asserter
	for prevGroup, prevMetrics := range prevExclude {
		a.exclude[prevGroup] = prevMetrics
	}

	if a.exclude[groupName] == nil {
		a.exclude[groupName] = map[string]bool{}
	}

	for _, e := range metricNames {
		a.exclude[groupName][e] = true
	}

	return a
}

// Silently returns an asserter that will not log optional or excepted metrics
func (a Asserter) Silently() Asserter {
	a.silent = true
	return a
}

// ExcludingOptional returns an asserter that will not fail, but log, metrics that are marked as optional in the SpecGroup.
func (a Asserter) ExcludingOptional() Asserter {
	a.excludeOptional = true
	return a
}

// Assert checks whether all metrics defined in the supplied groups are present, and fails the test if any is not.
func (a Asserter) Assert(t *testing.T) {
	t.Helper()

	if len(a.specGroups) == 0 {
		t.Fatalf("cannot assert empty specGroups, did you forget Using()?")
	}

	// TODO: Consider paralleling if it's too slow.
	for groupName, group := range a.specGroups {
		exclusions, excludeGroup := a.exclude[groupName]
		if excludeGroup && len(exclusions) == 0 {
			// Group exclusion present with any metric, meaning we exclude the whole group
			t.Logf("skipping excluded group %q", groupName)
			continue
		}

		// Integration will contain many entities, but we are only interested in the one corresponding to this group.
		entities := entitiesFor(a.entities, groupName)
		if entities == nil {
			t.Fatalf("could not find any entity for specGroup %q", groupName)
		}

		for _, spec := range group.Specs {
			for _, entity := range entities {
				if entityHas(entity, spec.Name, spec.Type) {
					continue
				}

				if spec.Optional && a.excludeOptional {
					if !a.silent {
						t.Logf("optional metric %q not found in entity %q", spec.Name, entity.Metadata.Name)
					}
					continue
				}

				if exclusions[spec.Name] {
					if !a.silent {
						t.Logf("excluded metric %q not found in entity %q", spec.Name, entity.Metadata.Name)
					}
					continue
				}

				t.Errorf("metric %q not found in entity %q %q", spec.Name, entity.Metadata.Namespace, entity.Metadata.Name)
				t.Failed()
			}
		}
	}
}

// entitiesFor heuristically finds the entity associated to a spec group name.
func entitiesFor(entities []*integration.Entity, pseudotype string) []*integration.Entity {
	var appropriateEntities []*integration.Entity
	for _, e := range entities {
		if strings.Contains(strings.ToLower(e.Metadata.Namespace), strings.ToLower(pseudotype)) {
			appropriateEntities = append(appropriateEntities, e)
		}
	}

	return appropriateEntities
}

// entityHas returns true if supplied entity has metric m with type _similar_ to mType, false otherwise.
func entityHas(e *integration.Entity, m string, mType metric.SourceType) bool {
	// Wildcard metrics are ignored.
	// TODO: Improve this and check matching glob patterns.
	if strings.HasSuffix(m, "*") {
		return true
	}

	for _, ms := range e.Metrics {
		entityMetric, found := ms.Metrics[m]
		if !found {
			continue
		}

		// Check if metricType is an attribute but metric is not a string
		_, isString := entityMetric.(string)
		if isString && mType != metric.ATTRIBUTE {
			continue
		}

		if !isString && mType == metric.ATTRIBUTE {
			continue
		}

		return true
	}

	return false
}
