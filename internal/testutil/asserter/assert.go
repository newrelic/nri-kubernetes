package asserter

import (
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v3/internal/testutil/asserter/exclude"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const entityNamespaceSeparator = ":"

// Asserter is a helper for checking whether an integration contains all the metrics defined in a specGroup.
// It provides a chainable API, with each call returning a copy of the asserter. This way, successive calls to the
// chainable methods do not modify the previous Asserter, allowing to reuse the chain as a test fans out.
// Asserter is safe to use concurrently.
type Asserter struct {
	entities       []*integration.Entity
	specGroups     definition.SpecGroups
	excludedGroups []string
	exclude        []exclude.Func
	silent         bool
	groupAliases   map[string]string
}

// New returns an empty asserter.
func New() Asserter {
	return Asserter{}
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

// Excluding returns an asserter that will not fail for a missing metric for which any of the supplied Func
// return true.
// For ignoring whole spec groups, use ExcludingGroups instead.
// Missing metrics are still logged, unless Silently is used.
func (a Asserter) Excluding(excludeFuncs ...exclude.Func) Asserter {
	exclude := make([]exclude.Func, len(a.exclude), len(a.exclude)+len(excludeFuncs))
	copy(exclude, a.exclude)

	a.exclude = append(exclude, excludeFuncs...)
	return a
}

// ExcludingGroups returns an asserter configured to completely exclude the supplied groups.
// Unlike Excluding, ExcludingGroups will ignore the group _before_ checking if there are any entities at all matching
// the group, an scenario that would make the asserter fail if the group is not excluded this way.
func (a Asserter) ExcludingGroups(groupNames ...string) Asserter {
	excludedGroups := make([]string, len(a.excludedGroups), len(a.excludedGroups)+len(groupNames))
	copy(excludedGroups, a.excludedGroups)

	a.excludedGroups = append(excludedGroups, groupNames...)
	return a
}

// Silently returns an asserter that will not log optional or excepted metrics
func (a Asserter) Silently() Asserter {
	a.silent = true
	return a
}

// AliasingGroups returns an asserter configured with an alias for groups so it is possible
// to look for entities with a different group name.
func (a Asserter) AliasingGroups(aliases map[string]string) Asserter {
	a.groupAliases = aliases
	return a
}

// Assert checks whether all metrics defined in the supplied groups are present, and fails the test if any is not.
// Assert will fail the test if:
//   - No entity at all exists with a type matching a specGroup, unless this specGroup is ignored using ExcludingGroups.
//   - Any entity whose type matches a specGroup lacks any metric defined in the specGroup, unless any Func returns
//     true for that particular groupName, metric, and entity.
func (a Asserter) Assert(t *testing.T) {
	t.Helper()

	if len(a.specGroups) == 0 {
		t.Fatalf("cannot assert empty specGroups, did you forget Using()?")
	}

	// TODO: Consider paralleling if it's too slow.
	for groupName, group := range a.specGroups {
		if a.shouldExcludeGroup(groupName) {
			t.Logf("Excluding specGroup %q", groupName)
			continue
		}

		// Integration will contain many entities, but we are only interested in the one corresponding to this group.
		pseudotype := groupName
		if alias := a.groupAliases[groupName]; alias != "" {
			pseudotype = alias
		}
		entities := entitiesFor(a.entities, pseudotype)
		if entities == nil {
			t.Fatalf("could not find any entity for specGroup %q (%q)", groupName, pseudotype)
		}

		for _, spec := range group.Specs {
			for _, entity := range entities {
				if EntityMetricTypeIs(entity, spec.Name, spec.Type) {
					continue
				}

				if a.shouldExclude(groupName, &spec, entity) {
					if !a.silent {
						t.Logf("excluded metric %q not found in entity %q (%s)", spec.Name, entity.Metadata.Name, entity.Metadata.Namespace)
					}
					continue
				}

				t.Errorf("metric %q not found in entity %q (%s)", spec.Name, entity.Metadata.Name, entity.Metadata.Namespace)
				t.Failed()
			}
		}
	}
}

// shouldExclude checks all configured Func in the asserter and returns true if any of them return true.
func (a *Asserter) shouldExclude(group string, spec *definition.Spec, ent *integration.Entity) bool {
	for _, exclusion := range a.exclude {
		if exclusion(group, spec, ent) {
			return true
		}
	}

	return false
}

// shouldExcludeGroup returns true if the specified group is present in Asserter.excludedGroups.
func (a *Asserter) shouldExcludeGroup(group string) bool {
	for _, exclusion := range a.excludedGroups {
		if group == exclusion {
			return true
		}
	}

	return false
}

// entitiesFor heuristically finds the entity associated to a spec group name.
func entitiesFor(entities []*integration.Entity, pseudotype string) []*integration.Entity {
	var appropriateEntities []*integration.Entity
	for _, e := range entities {
		if specGroupNameMatch(e, pseudotype) {
			appropriateEntities = append(appropriateEntities, e)
		}
	}

	return appropriateEntities
}

// specGroupNameMatch returns true if the specGroupName match with the provided entity.
func specGroupNameMatch(entity *integration.Entity, specGroupName string) bool {
	chunks := strings.Split(entity.Metadata.Namespace, entityNamespaceSeparator)
	for _, chunk := range chunks {
		if chunk == specGroupName {
			return true
		}
	}
	return false
}

// entityMetric is a helper function that returns the first metric from an entity that matches the given name.
func entityMetric(e *integration.Entity, m string) interface{} {
	for _, ms := range e.Metrics {
		if entMetric, found := ms.Metrics[m]; found {
			return entMetric
		}
	}

	return nil
}

// EntityMetricIs returns true if the specified entity has a metric named metricName equal to metricValue.
func EntityMetricIs(e *integration.Entity, metricName string, metricValue interface{}) bool {
	// Wildcard metrics are ignored.
	// TODO: Improve this and check matching glob patterns.
	if strings.HasSuffix(metricName, "*") {
		return true
	}

	switch mv := metricValue.(type) {
	case string:
		emv := entityMetric(e, metricName)
		emvString, isString := emv.(string)
		if !isString {
			return false
		}

		return strings.EqualFold(emvString, mv)

	default:
		return entityMetric(e, metricName) == metricValue
	}
}

// EntityMetricTypeIs returns true if supplied entity has metric named metricName with type _similar_ to metricType.
func EntityMetricTypeIs(e *integration.Entity, metricName string, metricType metric.SourceType) bool {
	// Wildcard metrics are ignored.
	// TODO: Improve this and check matching glob patterns.
	if strings.HasSuffix(metricName, "*") {
		return true
	}

	em := entityMetric(e, metricName)
	if em == nil {
		return false
	}

	_, isString := em.(string)

	// Return true if metric is a string and metricType is metric.ATTRIBUTE, or
	// if metric type is not a string and metricType is anything other than metric.ATTRIBUTE.
	return (isString && metricType == metric.ATTRIBUTE) || (!isString && metricType != metric.ATTRIBUTE)
}
