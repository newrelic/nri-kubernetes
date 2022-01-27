package exclude

import (
	"strings"

	"github.com/newrelic/infra-integrations-sdk/integration"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// Func is a function that returns true if a particular metric (spec) should be excluded from being asserted on
// ent.
// If a Func returns true for a given group, metric (spec) and entity, Asserter will not fail even if the metric
// is not found.
type Func func(group string, spec *definition.Spec, ent *integration.Entity) bool

// Exclude returns a Func that returns true if all the supplied ExcludeFuncs return true.
// Input ExcludeFuncs are evaluated in order, so this function makes easy to compose exclusion rules.
func Exclude(funcs ...Func) Func {
	return func(group string, spec *definition.Spec, ent *integration.Entity) bool {
		for _, f := range funcs {
			if !f(group, spec, ent) {
				return false
			}
		}
		return true
	}
}

// Optional returns a Func that excludes metrics marked as Optional.
func Optional() Func {
	return func(group string, spec *definition.Spec, ent *integration.Entity) bool {
		return spec.Optional
	}
}

// Groups returns a Func that will exclude a metric if group matches the supplied group.
func Groups(groups ...string) Func {
	return func(g string, spec *definition.Spec, ent *integration.Entity) bool {
		for _, group := range groups {
			if g == group {
				return true
			}
		}

		return false
	}
}

// Metrics returns a Func that excludes the specified metric names.
func Metrics(metricNames ...string) Func {
	return func(g string, spec *definition.Spec, ent *integration.Entity) bool {
		for _, m := range metricNames {
			if strings.EqualFold(spec.Name, m) {
				return true
			}
		}

		return false
	}
}

// Dependent receives a map between a metric name and other metric names that depend on it, and returns an
// Func that will exclude the dependencies if the dependent is not present.
func Dependent(dependencies map[string][]string) Func {
	return func(group string, spec *definition.Spec, ent *integration.Entity) bool {
		for parent, children := range dependencies {
			for _, ms := range ent.Metrics {
				if _, hasParent := ms.Metrics[parent]; hasParent {
					continue
				}

				for _, child := range children {
					if spec.Name == child {
						return true
					}
				}
			}
		}

		return false
	}
}
