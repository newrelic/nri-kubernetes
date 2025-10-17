package definition

import (
	"fmt"
)

// RawValue is just any value from a raw metric.
type RawValue interface{}

// RawMetrics is a map of RawValue indexed by metric name.
type RawMetrics map[string]RawValue

// FetchedValue is just any value from an already fetched metric.
type FetchedValue interface{}

// FetchedValues is a map of FetchedValue indexed by metric name.
type FetchedValues map[string]FetchedValue

// FetchFunc fetches values or values from raw metric groups.
// Return FetchedValues if you want to prototype metrics.
type FetchFunc func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error)

// FilteredValue is the filtered value of an already fetched metric.
type FilteredValue interface{}

// FilterFunc applies a filtering function on the raw fetchedValue.
// Return FilteredValue if the filter was successfully applied or error otherwise.
type FilterFunc func(value FetchedValue, groupLabel, entityID string, groups RawGroups) (FilteredValue, error)

// RawGroups are grouped raw metrics.
// map[entityType][entityName][metricName]metricValue as interface{}.
type RawGroups map[string]map[string]RawMetrics

// TransformFunc transforms a FetchedValue.
type TransformFunc func(FetchedValue) (FetchedValue, error)

// FromRaw fetches metrics from raw metrics. Is the most simple use case.
func FromRaw(metricKey string) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		group, ok := groups[groupLabel]
		if !ok {
			return nil, fmt.Errorf("group %q not found", groupLabel)
		}

		entity, ok := group[entityID]
		if !ok {
			return nil, fmt.Errorf("entity %q not found", entityID)
		}

		value, ok := entity[metricKey]
		if !ok {
			return nil, fmt.Errorf("metric %q not found", metricKey)
		}

		return value, nil
	}
}

// Transform return a new FetchFunc that applies the transformFunc to the result of the fetchFunc passed as argument.
func Transform(fetchFunc FetchFunc, transformFunc TransformFunc) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		fetchedVal, err := fetchFunc(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}
		return transformFunc(fetchedVal)
	}
}

// TransformAndFilter return a new FetchFunc that first applies a TransformFunc to the result of the fetchFunc passed as argument.
// It then applies the FilterFunc to the result of the TransformFunc if the transform was successfully applied.
func TransformAndFilter(fetchFunc FetchFunc, transformFunc TransformFunc, filterFunc FilterFunc) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		fetchedVal, err := fetchFunc(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}
		fetchedVal, err = transformFunc(fetchedVal)
		if err != nil {
			return nil, err
		}
		return filterFunc(fetchedVal, groupLabel, entityID, groups)
	}
}
