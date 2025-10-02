package populator

import (
	"fmt"
	"strconv"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

// processingUnit holds all the pre-calculated information needed to create and populate a single entity.
type processingUnit struct {
	entityID   string
	entityType string
	rawMetrics definition.RawMetrics
}

// IntegrationPopulator is the main orchestrator that populates an integration.Integration
// object from the grouped metric data.
func IntegrationPopulator(config *definition.IntegrationPopulateConfig) (bool, []error) {
	var populated bool
	var errs []error

	for groupLabel, entities := range config.Groups {
		for entityID, rawMetrics := range entities {
			specGroup, ok := config.Specs[groupLabel]
			if !ok {
				continue
			}

			var extraAttributes []attribute.Attribute
			if config.Filterer != nil {
				if nsGetter := specGroup.NamespaceGetter; nsGetter != nil {
					ns := nsGetter(rawMetrics)
					if groupLabel != definition.NamespaceGroup {
						if !config.Filterer.IsAllowed(ns) {
							continue // This is the filter for non-namespace groups.
						}
					} else {
						isFiltered := strconv.FormatBool(!config.Filterer.IsAllowed(ns))
						extraAttributes = []attribute.Attribute{attribute.Attr(definition.NamespaceFilteredLabel, isFiltered)}
					}
				}
			}

			unitsToProcess, err := prepareProcessingUnits(config, groupLabel, entityID, rawMetrics)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			for _, unit := range unitsToProcess {
				e, err := config.Integration.Entity(unit.entityID, unit.entityType)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				extraAttributes = append(
					extraAttributes,
					attribute.Attr("clusterName", config.ClusterName),
					attribute.Attr("displayName", e.Metadata.Name),
				)

				// Add entity attributes, which will propagate to all metric.Sets.
				// This was previously (on sdk v2) done by msManipulators.
				e.AddAttributes(
					extraAttributes...,
				)

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

				groupsForThisEntity := definition.RawGroups{
					groupLabel: {unit.entityID: unit.rawMetrics},
				}

				wasPopulated, populateErrs := metricSetPopulate(ms, groupLabel, unit.entityID, groupsForThisEntity, config.Specs)
				if len(populateErrs) > 0 {
					for _, err := range populateErrs {
						errs = append(errs, fmt.Errorf("error populating metric for entity ID %s: %s", entityID, err))
					}
				}
				if wasPopulated {
					populated = true
				}
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

// prepareProcessingUnits prepares a slice of one or more processingUnits.
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
			units = append(units, processingUnit{
				entityID:   fmt.Sprintf("%s_%s", entityID, subGroupKey),
				entityType: entityType,
				rawMetrics: subGroupMetrics,
			})
		}
		return units, nil
	}

	finalEntityID := entityID // Start with the raw ID from the grouper.

	if generator := specGroup.IDGenerator; generator != nil {
		// The IDGenerator uses the raw entityID for its lookup.
		generatedID, err := generator(groupLabel, entityID, config.Groups)
		if err != nil {
			return nil, fmt.Errorf("error generating entity ID for %s: %s", entityID, err)
		}
		finalEntityID = generatedID // Store the new ID for final use.
	}

	var entityType string
	if generatorType := specGroup.TypeGenerator; generatorType != nil {
		generatedType, err := generatorType(groupLabel, entityID, config.Groups, config.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("error generating entity type for %s: %s", entityID, err)
		}
		entityType = generatedType
	}

	return []processingUnit{
		{
			entityID:   finalEntityID,
			entityType: entityType,
			rawMetrics: rawMetrics,
		},
	}, nil

	return []processingUnit{
		{
			entityID:   entityID,
			entityType: entityType,
			rawMetrics: rawMetrics,
		},
	}, nil
}

// splitGroup takes a RawMetrics map that contains a slice of metrics and splits
// it into a map of smaller RawMetrics, partitioned by the unique values of the splitByLabel.
func splitGroup(rawMetrics definition.RawMetrics, sliceMetricName, groupLabel, splitByLabel string) (map[string]definition.RawMetrics, error) {
	if sliceMetricName == "" {
		sliceMetricName = groupLabel
	}

	mainMetricSlice, ok := rawMetrics[sliceMetricName].([]prometheus.Metric)
	if !ok {
		return nil, fmt.Errorf("group %q with SplitByLabel requires a slice of metrics for key %q, but found %T", groupLabel, sliceMetricName, rawMetrics[sliceMetricName])
	}

	subGroups := make(map[string]definition.RawMetrics)
	for _, metric := range mainMetricSlice {
		splitValue, ok := metric.Labels[splitByLabel]
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

		// Get or create the metric slice within the sub-group's map.
		slice, ok := subGroupMetrics[sliceMetricName].([]prometheus.Metric)
		if !ok {
			slice = make([]prometheus.Metric, 0)
		}

		// Append the current metric to the slice for this sub-group.
		subGroupMetrics[sliceMetricName] = append(slice, metric)
	}

	return subGroups, nil
}

func metricSetPopulate(ms *metric.Set, groupLabel, entityID string, groups definition.RawGroups, specs definition.SpecGroups) (populated bool, errs []error) {
	for _, ex := range specs[groupLabel].Specs {
		val, err := ex.ValueFunc(groupLabel, entityID, groups)
		if err != nil {
			if !ex.Optional {
				errs = append(errs, fmt.Errorf("cannot fetch value for metric %q: %w", ex.Name, err))
			}
			continue
		}

		if val == nil {
			continue // Skip nil values
		}

		if multiple, ok := val.(definition.FetchedValues); ok {
			for k, v := range multiple {
				err := ms.SetMetric(k, v, ex.Type)
				if err != nil {
					if !ex.Optional {
						errs = append(errs, fmt.Errorf("cannot set metric %s with value %v in metric set, %s", k, v, err))
					}
					continue
				}
				populated = true
			}
		} else {
			err := ms.SetMetric(ex.Name, val, ex.Type)
			if err != nil {
				if !ex.Optional {
					errs = append(errs, fmt.Errorf("cannot set metric %s with value %v in metric set, %s", ex.Name, val, err))
				}
				continue
			}
			populated = true
		}
	}

	return
}

func populateCluster(i *integration.Integration, clusterName string, k8sVersion fmt.Stringer) error {
	e, err := i.Entity(clusterName, "k8s:cluster")
	if err != nil {
		return err
	}
	ms := e.NewMetricSet("K8sClusterSample")

	err = e.Inventory.SetItem("cluster", "name", clusterName)
	if err != nil {
		return err
	}

	err = ms.SetMetric("clusterName", clusterName, metric.ATTRIBUTE)
	if err != nil {
		return err
	}

	k8sVersionStr := k8sVersion.String()
	err = e.Inventory.SetItem("cluster", "k8sVersion", k8sVersionStr)
	if err != nil {
		return err
	}

	err = e.Inventory.SetItem("cluster", "newrelic.integrationVersion", i.IntegrationVersion)
	if err != nil {
		return err //nolint: wrapcheck
	}

	err = e.Inventory.SetItem("cluster", "newrelic.integrationName", i.Name)
	if err != nil {
		return err //nolint: wrapcheck
	}

	return ms.SetMetric("clusterK8sVersion", k8sVersionStr, metric.ATTRIBUTE)
}
