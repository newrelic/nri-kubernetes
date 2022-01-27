package metric

import (
	"errors"
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// FromRawWithFallbackToDefaultInterface fetches network metrics from the raw
// groups, if the metric is not present it tries to find the default interface
// network metrics and gets the required metric from there.
func FromRawWithFallbackToDefaultInterface(metricKey string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		g, ok := groups[groupLabel]
		if !ok {
			return nil, errors.New("group not found")
		}

		e, ok := g[entityID]
		if !ok {
			return nil, errors.New("entity not found")
		}

		value, ok := e[metricKey]
		if ok {
			return value, nil
		}

		defaultInterface, err := getDefaultInterface(groups)
		if err != nil {
			return nil, fmt.Errorf(
				"metric not found and default interface fallback failed: %w", err)
		}

		metric, err := getMetricFromDefaultInterface(defaultInterface, metricKey, e)
		if err != nil {
			return nil, fmt.Errorf(
				"metric not found and default interface fallback failed: %w", err)
		}
		return metric, nil
	}
}

// getDefaultInterface from group["network"]["interfaces"]["default"]
func getDefaultInterface(groups definition.RawGroups) (string, error) {
	network, ok := groups["network"]
	if !ok {
		return "", errors.New("network group not found")
	}
	networkInterfaces, ok := network["interfaces"]
	if !ok {
		return "", errors.New("network interfaces attribute not found")
	}
	defaultInterfaceI, ok := networkInterfaces["default"]
	if !ok {
		return "", errors.New("default interface not found")
	}
	defaultInterface, ok := defaultInterfaceI.(string)
	if !ok {
		return "", errors.New("default interface is not a valid interface name")
	}
	if defaultInterface == "" {
		return "", errors.New("default interface not set")
	}

	return defaultInterface, nil
}

// getMetricFromDefaultInterface returns the value of metricKey related to
// the defaultInterface in the given raw metrics
func getMetricFromDefaultInterface(defaultInterface, metricKey string, m definition.RawMetrics) (definition.FetchedValue, error) {
	interfacesI, ok := m["interfaces"]
	if !ok {
		return nil, errors.New("interfaces metrics not found")
	}
	interfaces, ok := interfacesI.(map[string]definition.RawMetrics)
	if !ok {
		return nil, errors.New("wrong format for interfaces metrics")
	}

	for interfaceName, i := range interfaces {
		if interfaceName != defaultInterface {
			continue
		}
		value, ok := i[metricKey]
		if !ok {
			return nil, errors.New("metric not found for default interface")
		}
		return value, nil
	}
	return nil, errors.New("default interface metrics not found")
}
