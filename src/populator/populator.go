package populator

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

var (
	ErrGroupNotASlice = errors.New("group requires a slice of metrics")
	ErrGenerateID     = errors.New("could not generate entity ID")
	ErrGenerateType   = errors.New("could not generate entity type")
	ErrSetMetric      = errors.New("could not set metric")
)

// processingUnit holds all the pre-calculated information needed to create and populate a single entity.
type processingUnit struct {
	originalEntityID string // The original entity ID from the grouper, used for metric lookups
	entityID         string // The final entity ID (after IDGenerator), used for entity creation
	entityType       string
	rawMetrics       definition.RawMetrics
}

// IntegrationPopulator is the main orchestrator that populates an integration.Integration
// object from the grouped metric data. It prepares "processing units" for each entity
// or sub-entity and then populates them.
func IntegrationPopulator(config *definition.IntegrationPopulateConfig) (bool, []error) {
	var populated bool
	var errs []error

	for groupLabel, entities := range config.Groups {
		for entityID, rawMetrics := range entities {
			specGroup, ok := config.Specs[groupLabel]
			if !ok {
				continue
			}

			extraAttributes, skip := filterGroup(config, specGroup, groupLabel, rawMetrics)
			if skip {
				continue
			}

			unitsToProcess, err := prepareProcessingUnits(config, groupLabel, entityID, rawMetrics)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			pop, perr := processEntities(unitsToProcess, config, specGroup, groupLabel, extraAttributes)
			if len(perr) > 0 {
				errs = append(errs, perr...)
			}
			if pop {
				populated = true
			}
		}
	}

	if populated {
		if err := populateCluster(config.Integration, config.ClusterName, config.K8sVersion); err != nil {
			errs = append(errs, err)
		}
	}

	return populated, errs
}

// filterGroup checks if an entity group should be filtered by namespace.
// It returns true if the group should be filtered. For namespace-group entities,
// it returns extra attributes to be added.
func filterGroup(
	config *definition.IntegrationPopulateConfig,
	specGroup definition.SpecGroup,
	groupLabel string,
	rawMetrics definition.RawMetrics,
) ([]attribute.Attribute, bool) {
	if config.Filterer == nil || specGroup.NamespaceGetter == nil {
		return nil, false
	}
	ns := specGroup.NamespaceGetter(rawMetrics)
	if groupLabel != definition.NamespaceGroup {
		if !config.Filterer.IsAllowed(ns) {
			return nil, true // skip
		}
		return nil, false
	}
	isFiltered := strconv.FormatBool(!config.Filterer.IsAllowed(ns))
	attributes := []attribute.Attribute{attribute.Attr(definition.NamespaceFilteredLabel, isFiltered)}
	return attributes, false
}

// processEntities handles the creation and population of a single entity (or sub-entity).
func processEntities(unitsToProcess []processingUnit, config *definition.IntegrationPopulateConfig, specGroup definition.SpecGroup, groupLabel string, extraAttributes []attribute.Attribute) (bool, []error) {
	var populated bool
	var errs []error

	for _, unit := range unitsToProcess {
		e, err := config.Integration.Entity(unit.entityID, unit.entityType)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		attrs := make([]attribute.Attribute, len(extraAttributes), len(extraAttributes)+2)
		copy(attrs, extraAttributes)
		attrs = append(attrs,
			attribute.Attr("clusterName", config.ClusterName),
			attribute.Attr("displayName", e.Metadata.Name),
		)
		e.AddAttributes(attrs...)

		msTypeGuesser := config.MsTypeGuesser
		if customGuesser := specGroup.MsTypeGuesser; customGuesser != nil {
			msTypeGuesser = customGuesser
		}
		msType, err := msTypeGuesser(groupLabel)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ms := e.NewMetricSet(msType)

		groupsForThisEntity := definition.RawGroups{}
		for groupName, groupValue := range config.Groups {
			groupsForThisEntity[groupName] = groupValue
		}
		// Use originalEntityID for RawGroups key to match grouper's format
		groupsForThisEntity[groupLabel] = map[string]definition.RawMetrics{unit.originalEntityID: unit.rawMetrics}

		// Use originalEntityID for metric lookups (InheritAllLabelsFrom needs this)
		wasPopulated, populateErrs := metricSetPopulate(ms, groupLabel, unit.originalEntityID, groupsForThisEntity, config.Specs)
		if len(populateErrs) > 0 {
			for _, err := range populateErrs {
				errs = append(errs, fmt.Errorf("error populating metric for entity ID %s: %w", unit.entityID, err))
			}
		}
		if wasPopulated {
			populated = true
		}
	}
	return populated, errs
}

// prepareProcessingUnits takes a raw entity group and, based on its SpecGroup rules,
// returns a slice of one or more processingUnits. This is the core of the sub-grouping
// logic: it either prepares a single unit for a standard entity or multiple units if
// the group is configured to be split by a label.
func prepareProcessingUnits(config *definition.IntegrationPopulateConfig, groupLabel, entityID string, rawMetrics definition.RawMetrics) ([]processingUnit, error) {
	specGroup := config.Specs[groupLabel]

	if specGroup.SplitByLabel != "" {
		subGroups, err := splitGroup(rawMetrics, specGroup.SliceMetricName, groupLabel, specGroup.SplitByLabel)
		if err != nil {
			return nil, err
		}
		entityType, err := specGroup.TypeGenerator(groupLabel, entityID, config.Groups, config.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("error generating entity type for parent %s: %w", entityID, err)
		}
		units := make([]processingUnit, 0, len(subGroups))
		for subGroupKey, subGroupMetrics := range subGroups {
			subEntityID := fmt.Sprintf("%s_%s", entityID, subGroupKey)
			units = append(units, processingUnit{
				originalEntityID: entityID,    // Use parent's original ID for metric lookups
				entityID:         subEntityID, // Use sub-entity ID for entity creation
				entityType:       entityType,
				rawMetrics:       subGroupMetrics,
			})
		}
		return units, nil
	}

	finalEntityID := entityID // Start with the raw ID from the grouper.

	if generator := specGroup.IDGenerator; generator != nil {
		// The IDGenerator uses the raw entityID for its lookup.
		generatedID, err := generator(groupLabel, entityID, config.Groups)
		if err != nil {
			return nil, fmt.Errorf("%w for %s: %w", ErrGenerateID, entityID, err)
		}
		finalEntityID = generatedID // Store the new ID for final use.
	}

	var entityType string
	if generatorType := specGroup.TypeGenerator; generatorType != nil {
		generatedType, err := generatorType(groupLabel, entityID, config.Groups, config.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("%w for %s: %w", ErrGenerateType, entityID, err)
		}
		entityType = generatedType
	}

	return []processingUnit{
		{
			originalEntityID: entityID,      // Store original for metric lookups
			entityID:         finalEntityID, // Use final for entity creation
			entityType:       entityType,
			rawMetrics:       rawMetrics,
		},
	}, nil
}

// splitGroup takes a RawMetrics map that contains a slice of metrics and partitions it
// into a map of smaller RawMetrics based on the unique values of the 'splitByLabel'.
// Shared metrics from the original RawMetrics (like _created, _labels) are copied
// to each new sub-group to provide context.
// This allows creating multiple metrics from a single parent
// Example:
//
// Given a RawMetrics map for a ResourceQuota:
//
//	{
//	  "kube_resourcequota_created": Metric{...},
//	  "kube_resourcequota": []Metric{
//	    {Labels: {"resource": "pods", "type": "hard"}},
//	    {Labels: {"resource": "secrets", "type": "hard"}},
//	    {Labels: {"resource": "pods", "type": "used"}},
//	  },
//	}
//
// Calling splitGroup(..., "kube_resourcequota", ..., "resource") will return:
//
//	{
//	  "pods": {
//	    "kube_resourcequota_created": Metric{...}, // Copied
//	    "kube_resourcequota": []Metric{
//	      {Labels: {"resource": "pods", "type": "hard"}},
//	      {Labels: {"resource": "pods", "type": "used"}},
//	    },
//	  },
//	  "secrets": {
//	    "kube_resourcequota_created": Metric{...}, // Copied
//	    "kube_resourcequota": []Metric{
//	      {Labels: {"resource": "secrets", "type": "hard"}},
//	    },
//	  },
//	}
func splitGroup(rawMetrics definition.RawMetrics, sliceMetricName, groupLabel, splitByLabel string) (map[string]definition.RawMetrics, error) {
	if sliceMetricName == "" {
		sliceMetricName = groupLabel
	}

	mainMetricSlice, ok := rawMetrics[sliceMetricName].([]prometheus.Metric)
	if !ok {
		return nil, fmt.Errorf("group %q key %q: %w", groupLabel, sliceMetricName, ErrGroupNotASlice)
	}

	subGroups := make(map[string]definition.RawMetrics)
	for _, m := range mainMetricSlice {
		splitValue, ok := m.Labels[splitByLabel]
		if !ok {
			continue // Skip metrics that don't have the label to split by.
		}

		// Get or create the RawMetrics map for this sub-group key (e.g., "secrets").
		subGroupMetrics, exists := subGroups[splitValue]
		if !exists {
			subGroupMetrics = make(definition.RawMetrics)
			// Copy shared metrics (like _created, _labels) from the parent group just once.
			for k, v := range rawMetrics {
				if k != sliceMetricName { // Do not copy the large, unfiltered slice itself.
					subGroupMetrics[k] = v
				}
			}
			subGroups[splitValue] = subGroupMetrics
		}

		// Get or create the m slice within the sub-group's map.
		slice, ok := subGroupMetrics[sliceMetricName].([]prometheus.Metric)
		if !ok {
			slice = make([]prometheus.Metric, 0)
		}

		// Append the current m to the slice for this sub-group.
		subGroupMetrics[sliceMetricName] = append(slice, m)
	}

	return subGroups, nil
}

// metricSetPopulate acts as a dispatcher, populating a metric set based on the spec definitions.
func metricSetPopulate(ms *metric.Set, groupLabel, entityID string, groups definition.RawGroups, specs definition.SpecGroups) (bool, []error) {
	var populated bool
	var errs []error

	// 1. Look up the specific SpecGroup from the map using the groupLabel.
	specGroup, ok := specs[groupLabel]
	if !ok {
		// If no spec is defined for this group, there's nothing to do.
		return false, nil
	}

	// 2. The rest of the logic remains the same, using 'specGroup' which we just found.
	for _, spec := range specGroup.Specs {
		val, err := spec.ValueFunc(groupLabel, entityID, groups)
		if err != nil {
			if !spec.Optional {
				errs = append(errs, fmt.Errorf("cannot fetch value for metric %q: %w", spec.Name, err))
			}
			continue
		}
		if val == nil {
			continue
		}

		p, e := populateValue(ms, &spec, val)
		if e != nil && !spec.Optional {
			errs = append(errs, fmt.Errorf("populating entity %q: %w", entityID, e))
		}
		if p {
			populated = true
		}
	}
	return populated, errs
}

// populateValue is a helper that adds a fetched value to a metric set by determining its type.
func populateValue(ms *metric.Set, spec *definition.Spec, val definition.FetchedValue) (bool, error) {
	switch v := val.(type) {
	case definition.FetchedValues:
		return populateMetricsFromMap(ms, v, spec.Type)
	default:
		return populateSingleMetric(ms, spec.Name, v, spec.Type)
	}
}

// populateMetricsFromMap adds multiple metrics that all share a single type from the spec.
func populateMetricsFromMap(ms *metric.Set, metrics definition.FetchedValues, sourceType metric.SourceType) (bool, error) {
	if len(metrics) == 0 {
		return false, nil
	}
	for k, v := range metrics {
		if err := ms.SetMetric(k, v, sourceType); err != nil {
			return false, fmt.Errorf("%w %q: %w", ErrSetMetric, k, err)
		}
	}
	return true, nil
}

// populateSingleMetric adds a single metric to the metric set.
func populateSingleMetric(ms *metric.Set, name string, value interface{}, sourceType metric.SourceType) (bool, error) {
	if err := ms.SetMetric(name, value, sourceType); err != nil {
		return false, fmt.Errorf("%w %q: %w", ErrSetMetric, name, err)
	}
	return true, nil
}

// populateCluster fills cluster-level data.
func populateCluster(i *integration.Integration, clusterName string, k8sVersion fmt.Stringer) error {
	e, err := i.Entity(clusterName, "k8s:cluster")
	if err != nil {
		// Add context to the error from the SDK.
		return fmt.Errorf("could not create cluster entity: %w", err)
	}
	ms := e.NewMetricSet("K8sClusterSample")

	err = e.Inventory.SetItem("cluster", "name", clusterName)
	if err != nil {
		return fmt.Errorf("could not set cluster name in inventory: %w", err)
	}

	err = ms.SetMetric("clusterName", clusterName, metric.ATTRIBUTE)
	if err != nil {
		return fmt.Errorf("could not set clusterName metric: %w", err)
	}

	k8sVersionStr := k8sVersion.String()
	err = e.Inventory.SetItem("cluster", "k8sVersion", k8sVersionStr)
	if err != nil {
		return fmt.Errorf("could not set k8sVersion in inventory: %w", err)
	}

	err = e.Inventory.SetItem("cluster", "newrelic.integrationVersion", i.IntegrationVersion)
	if err != nil {
		return fmt.Errorf("could not set integration version in inventory: %w", err)
	}

	err = e.Inventory.SetItem("cluster", "newrelic.integrationName", i.Name)
	if err != nil {
		return fmt.Errorf("could not set integration name in inventory: %w", err)
	}

	if err = ms.SetMetric("clusterK8sVersion", k8sVersionStr, metric.ATTRIBUTE); err != nil {
		return fmt.Errorf("could not set clusterK8sVersion metric: %w", err)
	}

	return nil
}
