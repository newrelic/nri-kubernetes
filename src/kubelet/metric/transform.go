package metric

import (
	"errors"
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// PrefixFromMapInt does the same as OneMetricPerLabel but for map[string]int and with configurable prefix.
// We need two separate functions because we must return a map from string to a concrete type, as that type will be
// later asserted and checked.
func PrefixFromMapInt(prefix string) func(mapValue definition.FetchedValue) (definition.FetchedValue, error) {
	return func(value definition.FetchedValue) (definition.FetchedValue, error) {
		mapValue, ok := value.(map[string]int)
		if !ok {
			return value, fmt.Errorf("cannot make prefixes: value is not map[string]string")
		}

		prefixed := make(definition.FetchedValues, len(mapValue))
		for k, v := range mapValue {
			prefixed[fmt.Sprintf("%s%v", prefix, k)] = v
		}

		return prefixed, nil
	}
}

// OneMetricPerLabel transforms a map of labels to FetchedValues type,
// which will be converted later to one metric per label.
// It also prefix the labels with 'label.'
func OneMetricPerLabel(rawLabels definition.FetchedValue) (definition.FetchedValue, error) {
	labels, ok := rawLabels.(map[string]string)
	if !ok {
		return rawLabels, errors.New("error on creating kubelet label metrics")
	}

	modified := make(definition.FetchedValues, len(labels))
	for k, v := range labels {
		modified[fmt.Sprintf("label.%v", k)] = v
	}

	return modified, nil
}
