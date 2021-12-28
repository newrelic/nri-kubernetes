package asserter

import (
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v2/internal/testutil/asserter/exclude"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

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
	exclude := make([]exclude.Func, len(a.exclude)+len(excludeFuncs))
	copy(exclude, a.exclude)

	a.exclude = append(a.exclude, excludeFuncs...)

	return a
}

// ExcludingGroups returns an asserter configured to completely exclude the supplied groups.
// Unlike Excluding, ExcludingGroups will ignore the group _before_ checking if there are any entities at all matching
// the group, an scenario that would make the asserter fail if the group is not excluded this way.
func (a Asserter) ExcludingGroups(groupNames ...string) Asserter {
	excludeGroups := make([]string, len(a.excludedGroups)+len(groupNames))
	copy(excludeGroups, a.excludedGroups)

	a.excludedGroups = append(a.excludedGroups, groupNames...)

	return a
}

// Silently returns an asserter that will not log optional or excepted metrics
func (a Asserter) Silently() Asserter {
	a.silent = true
	return a
}

// Assert checks whether all metrics defined in the supplied groups are present, and fails the test if any is not.
// Assert will fail the test if:
// - No entity at all exists with a type matching a specGroup, unless this specGroup is ignored using ExcludingGroups.
// - Any entity whose type matches a specGroup lacks any metric defined in the specGroup, unless any Func returns
//   true for that particular groupName, metric, and entity.
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
		entities := entitiesFor(a.entities, groupName)
		if entities == nil {
			t.Fatalf("could not find any entity for specGroup %q", groupName)
		}

		for _, spec := range group.Specs {
			for _, entity := range entities {
				if EntityContains(entity, spec.Name, spec.Type) {
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
		if strings.Contains(strings.ToLower(e.Metadata.Namespace), strings.ToLower(pseudotype)) {
			appropriateEntities = append(appropriateEntities, e)
		}
	}

	return appropriateEntities
}

// entityMetric is a helper function that returns the first metric from an entity that matches the given name.
func entityMetric(e *integration.Entity, m string) interface{} {
	for _, ms := range e.Metrics {
		entityMetric, found := ms.Metrics[m]
		if found {
			return entityMetric
		}
	}

	return nil
}

// EntityHas returns true if the specified entity has a metric of name m equal to value.
func EntityHas(e *integration.Entity, m string, value interface{}) bool {
	// Wildcard metrics are ignored.
	// TODO: Improve this and check matching glob patterns.
	if strings.HasSuffix(m, "*") {
		return true
	}

	return entityMetric(e, m) == value
}

// EntityContains returns true if supplied entity has metric m with type _similar_ to mType, false otherwise.
func EntityContains(e *integration.Entity, m string, mType metric.SourceType) bool {
	// Wildcard metrics are ignored.
	// TODO: Improve this and check matching glob patterns.
	if strings.HasSuffix(m, "*") {
		return true
	}

	em := entityMetric(e, m)
	if em == nil {
		return false
	}

	// Check if metricType is an attribute but metric is not a string
	_, isString := em.(string)

	return (isString && mType == metric.ATTRIBUTE) || (!isString && mType != metric.ATTRIBUTE)
}
