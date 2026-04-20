package metric

import (
	"errors"
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// Error variables for lint compliance.
var (
	ErrNotMapStringString    = errors.New("cannot make prefixes: value is not map[string]string")
	ErrNotMapStringInterface = errors.New("cannot make prefixes: value is not map[string]interface{}")
)

// PrefixFromMapInt does the same as OneMetricPerLabel but for map[string]int and with configurable prefix.
// We need two separate functions because we must return a map from string to a concrete type, as that type will be
// later asserted and checked.
func PrefixFromMapInt(prefix string) func(mapValue definition.FetchedValue) (definition.FetchedValue, error) {
	return func(value definition.FetchedValue) (definition.FetchedValue, error) {
		mapValue, ok := value.(map[string]int)
		if !ok {
			return value, ErrNotMapStringString
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
// It also prefixes the labels with 'label.'.
//
//nolint:ireturn // Returns interface type by design for metric transformation.
func OneMetricPerLabel(rawLabels definition.FetchedValue) (definition.FetchedValue, error) {
	labels, ok := rawLabels.(map[string]string)
	if !ok {
		return rawLabels, ErrNotMapStringString
	}

	modified := make(definition.FetchedValues, len(labels))
	for k, v := range labels {
		modified[fmt.Sprintf("label.%v", k)] = v
	}

	return modified, nil
}

// PrefixFromMapAny transforms a map[string]interface{} to FetchedValues with a configurable prefix.
// This is useful for diagnostic metrics that have mixed types (strings, ints, bools, floats).
// The prefix is prepended to each key in the resulting FetchedValues.
// All values are converted to strings since ATTRIBUTE type metrics only accept strings.
func PrefixFromMapAny(prefix string) func(mapValue definition.FetchedValue) (definition.FetchedValue, error) {
	return func(value definition.FetchedValue) (definition.FetchedValue, error) {
		mapValue, ok := value.(map[string]interface{})
		if !ok {
			return value, fmt.Errorf("%w: got %T", ErrNotMapStringInterface, value)
		}

		prefixed := make(definition.FetchedValues, len(mapValue))
		for k, v := range mapValue {
			// Convert all values to strings since ATTRIBUTE type only accepts strings.
			var strVal string
			switch val := v.(type) {
			case string:
				strVal = val
			case bool:
				if val {
					strVal = "true"
				} else {
					strVal = "false"
				}
			default:
				strVal = fmt.Sprintf("%v", v)
			}
			prefixed[fmt.Sprintf("%s%v", prefix, k)] = strVal
		}

		return prefixed, nil
	}
}
