package definition

import (
	"errors"
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

// RawGroups are grouped raw metrics.
type RawGroups map[string]map[string]RawMetrics

// TransformFunc transforms a FetchedValue.
type TransformFunc func(FetchedValue) (FetchedValue, error)

// FromRaw fetches metrics from raw metrics. Is the most simple use case.
func FromRaw(metricKey string) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		g, ok := groups[groupLabel]
		if !ok {
			return nil, errors.New("group not found")
		}

		e, ok := g[entityID]
		if !ok {
			return nil, errors.New("entity not found")
		}

		value, ok := e[metricKey]
		if !ok {
			return nil, errors.New("metric not found")
		}

		return value, nil
	}
}

// Transform return a new FetchFunc that applies the transformFunc to the result of the fetchFunc passed as argument
func Transform(fetchFunc FetchFunc, transformFunc TransformFunc) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		fetchedVal, err := fetchFunc(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}
		return transformFunc(fetchedVal)
	}
}
