package metric

import (
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	log "github.com/sirupsen/logrus"
)

var (
	physicalInterfacePattern = regexp.MustCompile(`^(eth|ens|eno|enp)\d+`)
	cniInterfacePattern      = regexp.MustCompile(`^(eni|oci|azv|veth|cali|cni|pod-|lxc|docker|br-)`)
)

// Static errors for network metric operations.
var (
	ErrGroupNotFound                      = errors.New("group not found")
	ErrEntityNotFound                     = errors.New("entity not found")
	ErrInterfacesMetricsNotFound          = errors.New("interfaces metrics not found")
	ErrInterfacesWrongFormat              = errors.New("wrong format for interfaces metrics")
	ErrNetworkGroupNotFound               = errors.New("network group not found")
	ErrNetworkInterfacesAttributeNotFound = errors.New("network interfaces attribute not found")
	ErrDefaultInterfaceNotFound           = errors.New("default interface not found")
	ErrDefaultInterfaceInvalidType        = errors.New("default interface is not a valid interface name")
	ErrDefaultInterfaceNotSet             = errors.New("default interface not set")
	ErrMetricNotFoundForDefaultInterface  = errors.New("metric not found for default interface")
	ErrDefaultInterfaceMetricsNotFound    = errors.New("default interface metrics not found")
	ErrNoPhysicalNetworkInterfaces        = errors.New("no physical network interfaces found")
)

// FromRawWithFallbackToDefaultInterface fetches network metrics from the raw
// groups, with multiple fallback strategies:
// 1. Try top-level metric (e.g., n.Network.RxBytes).
// 2. Try default interface from routing table (only for node entities, from /host/proc/1/net/route).
// 3. Use heuristic to select primary interface (lowest-numbered physical interface).
func FromRawWithFallbackToDefaultInterface(metricKey string, cache *InterfaceCache) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		e, err := getEntityMetrics(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		// Step 1: Try top-level metric (e.g., n.Network.RxBytes directly)
		if value, ok := e[metricKey]; ok {
			return value, nil
		}

		// Step 2: Try cached interface
		if metric, found := tryCache(cache, entityID, metricKey, e); found {
			return metric, nil
		}

		// Step 3: Try routing table (nodes only) or heuristic (all entities)
		iface, err := resolveInterface(groupLabel, entityID, metricKey, e, groups)
		if err != nil {
			return nil, err
		}

		// Cache and return the metric
		if cache != nil {
			cache.Put(entityID, iface)
		}

		return getMetricFromInterface(iface, metricKey, e)
	}
}

// getEntityMetrics extracts entity metrics from raw groups.
func getEntityMetrics(groupLabel, entityID string, groups definition.RawGroups) (definition.RawMetrics, error) {
	g, ok := groups[groupLabel]
	if !ok {
		return nil, ErrGroupNotFound
	}

	e, ok := g[entityID]
	if !ok {
		return nil, ErrEntityNotFound
	}

	return e, nil
}

// tryCache attempts to fetch metric from cached interface.
//
//nolint:ireturn
func tryCache(cache *InterfaceCache, entityID, metricKey string, e definition.RawMetrics) (definition.FetchedValue, bool) {
	if cache == nil {
		return nil, false
	}

	cachedInterface, found := cache.Get(entityID)
	if !found {
		return nil, false
	}

	log.Debugf("Using cached interface '%s' for entity '%s' metric %s", cachedInterface, entityID, metricKey)
	metric, err := getMetricFromInterface(cachedInterface, metricKey, e)
	if err == nil {
		return metric, true
	}

	// Cached interface no longer exists in stats
	log.Debugf("Cached interface '%s' not found in current stats for entity '%s', re-resolving", cachedInterface, entityID)
	return nil, false
}

// resolveInterface determines the interface to use for metrics, trying routing table (nodes only) then heuristic.
func resolveInterface(groupLabel, entityID, metricKey string, e definition.RawMetrics, groups definition.RawGroups) (string, error) {
	// Try routing table for nodes only (pods/containers have their own network namespaces)
	if groupLabel == "node" {
		if iface, ok := tryRoutingTable(entityID, groupLabel, metricKey, e, groups); ok {
			return iface, nil
		}
	}

	// Fall back to heuristic
	return resolveInterfaceHeuristic(entityID, metricKey, e)
}

// tryRoutingTable attempts to resolve interface from routing table.
// Returns (interface, true) on success, ("", false) on expected failures that should fallback to heuristic.
func tryRoutingTable(entityID, groupLabel, metricKey string, e definition.RawMetrics, groups definition.RawGroups) (string, bool) {
	defaultInterface, err := getDefaultInterface(groups)
	if err != nil || defaultInterface == "" {
		// Routing table not available or empty - fallback to heuristic
		return "", false
	}

	_, err = getMetricFromInterface(defaultInterface, metricKey, e)
	if err == nil {
		return defaultInterface, true
	}

	// Routing table interface not found in stats - fallback to heuristic
	log.Debugf("Default interface '%s' found in routing table but not in stats for entity '%s' (group: %s, metric: %s). Trying heuristic.", defaultInterface, entityID, groupLabel, metricKey)
	return "", false
}

// resolveInterfaceHeuristic selects the primary interface using heuristic (lowest-numbered physical interface).
func resolveInterfaceHeuristic(entityID, metricKey string, e definition.RawMetrics) (string, error) {
	interfacesI, ok := e["interfaces"]
	if !ok {
		return "", ErrInterfacesMetricsNotFound
	}

	interfaces, ok := interfacesI.(map[string]definition.RawMetrics)
	if !ok {
		return "", ErrInterfacesWrongFormat
	}

	primaryInterface, err := selectPrimaryInterface(interfaces)
	if err != nil {
		return "", fmt.Errorf("could not determine primary interface: %w", err)
	}

	log.Debugf("Using heuristic-selected primary interface '%s' for entity '%s' metric %s", primaryInterface, entityID, metricKey)
	return primaryInterface, nil
}

// getDefaultInterface from group["network"]["interfaces"]["default"].
func getDefaultInterface(groups definition.RawGroups) (string, error) {
	network, ok := groups["network"]
	if !ok {
		return "", ErrNetworkGroupNotFound
	}
	networkInterfaces, ok := network["interfaces"]
	if !ok {
		return "", ErrNetworkInterfacesAttributeNotFound
	}
	defaultInterfaceI, ok := networkInterfaces["default"]
	if !ok {
		return "", ErrDefaultInterfaceNotFound
	}
	defaultInterface, ok := defaultInterfaceI.(string)
	if !ok {
		return "", ErrDefaultInterfaceInvalidType
	}
	if defaultInterface == "" {
		return "", ErrDefaultInterfaceNotSet
	}

	return defaultInterface, nil
}

// getMetricFromDefaultInterface returns the value of metricKey related to
// the defaultInterface in the given raw metrics.
//
//nolint:ireturn,unused
func getMetricFromDefaultInterface(defaultInterface, metricKey string, m definition.RawMetrics) (definition.FetchedValue, error) {
	interfacesI, ok := m["interfaces"]
	if !ok {
		return nil, ErrInterfacesMetricsNotFound
	}
	interfaces, ok := interfacesI.(map[string]definition.RawMetrics)
	if !ok {
		return nil, ErrInterfacesWrongFormat
	}

	for interfaceName, i := range interfaces {
		if interfaceName != defaultInterface {
			continue
		}
		value, ok := i[metricKey]
		if !ok {
			return nil, ErrMetricNotFoundForDefaultInterface
		}
		return value, nil
	}
	return nil, ErrDefaultInterfaceMetricsNotFound
}

// selectPrimaryInterface identifies the primary network interface using heuristics
// when routing table access is unavailable. It selects the lowest-numbered physical
// interface while excluding known CNI interface patterns.
func selectPrimaryInterface(interfaces map[string]definition.RawMetrics) (string, error) {
	candidates := make([]string, 0, len(interfaces))

	for name := range interfaces {
		// Must match physical interface pattern
		if !physicalInterfacePattern.MatchString(name) {
			continue
		}
		// Must not match CNI pattern
		if cniInterfacePattern.MatchString(name) {
			continue
		}
		candidates = append(candidates, name)
	}

	if len(candidates) == 0 {
		return "", ErrNoPhysicalNetworkInterfaces
	}

	// Sort alphabetically and return lowest-numbered
	sort.Strings(candidates)
	return candidates[0], nil
}

// getMetricFromInterface extracts a metric from a specific interface.
//
//nolint:ireturn
func getMetricFromInterface(interfaceName, metricKey string, m definition.RawMetrics) (definition.FetchedValue, error) {
	interfacesI, ok := m["interfaces"]
	if !ok {
		return nil, ErrInterfacesMetricsNotFound
	}
	interfaces, ok := interfacesI.(map[string]definition.RawMetrics)
	if !ok {
		return nil, ErrInterfacesWrongFormat
	}

	interfaceMetrics, ok := interfaces[interfaceName]
	if !ok {
		return nil, fmt.Errorf("interface %s not found", interfaceName) //nolint:err113
	}

	value, ok := interfaceMetrics[metricKey]
	if !ok {
		return nil, fmt.Errorf("metric %s not found for interface %s", metricKey, interfaceName) //nolint:err113
	}
	return value, nil
}
