package populator

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

// It includes logic to split a single group into multiple sub-entities based on the 'SplitByLabel' spec.
// IntegrationPopulator populates an integration with the given metrics and definition.
// It includes logic to split a single group into multiple sub-entities based on the 'SplitByLabel' spec.
func IntegrationPopulator(config *definition.IntegrationPopulateConfig) (bool, []error) {
	var populated bool
	var errs []error

	for groupLabel, entities := range config.Groups {
		for entityID, rawMetrics := range entities {
			// DEBUG: Print the top-level group and entity being processed.
			fmt.Printf("--- Processing Group: %s, EntityID: %s ---\n", groupLabel, entityID)

			specGroup, ok := config.Specs[groupLabel]
			if !ok {
				continue
			}

			// Namespace filtering logic (no changes here).
			if config.Filterer != nil {
				if nsGetter := specGroup.NamespaceGetter; nsGetter != nil {
					ns := nsGetter(rawMetrics)
					if groupLabel != definition.NamespaceGroup {
						if !config.Filterer.IsAllowed(ns) {
							continue
						}
					}
					// 'else' block for adding NamespaceFilteredLabel is omitted for brevity but would be here.
				}
			}

			// Check if this group needs to be split into sub-entities.
			if specGroup.SplitByLabel != "" {
				// DEBUG: Announce that we are entering the special sub-grouping path.
				fmt.Printf("Found 'SplitByLabel: %s'. Entering sub-grouping logic.\n", specGroup.SplitByLabel)
				// Determine which metric name to use for the slice.
				sliceMetricName := specGroup.SliceMetricName
				if sliceMetricName == "" {
					// Fallback to groupLabel if not specified, for simpler cases.
					sliceMetricName = groupLabel
				}

				// DEBUG: Inspect the contents of rawMetrics before we try to access it.
				fmt.Printf("Inspecting rawMetrics map for entity '%s':\n", entityID)
				for key, value := range rawMetrics {
					fmt.Printf("  - Key: %-30s | Type: %T\n", key, value)
				}

				mainMetricSlice, ok := rawMetrics[sliceMetricName].([]prometheus.Metric)
				if !ok {
					// DEBUG: Explicitly state that the type assertion failed.
					fmt.Printf("DEBUG: FATAL! Type assertion to []prometheus.Metric FAILED. The actual type was %T.\n", rawMetrics[groupLabel])

					errs = append(errs, fmt.Errorf("group %q with SplitByLabel requires a slice of metrics, but found %T", groupLabel, rawMetrics[groupLabel]))
					continue
				}

				// DEBUG: Show how many metrics we are about to split.
				fmt.Printf("Found %d metrics in the slice to split.\n", len(mainMetricSlice))

				subGroups := make(map[string]definition.RawMetrics)

				for _, metric := range mainMetricSlice {
					splitValue, ok := metric.Labels[specGroup.SplitByLabel]
					if !ok {
						continue
					}

					// Get or create the RawMetrics map for this sub-group key (e.g., "secrets").
					subGroupMetrics, exists := subGroups[splitValue]
					if !exists {
						subGroupMetrics = make(definition.RawMetrics)
						// Copy the shared metrics (like _created, _labels) just once when the sub-group is first created.
						for k, v := range rawMetrics {
							if k != sliceMetricName { // Do not copy the large, unfiltered slice.
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

				// 2. Generate the Entity Type ONCE, using the original entityID for context.
				generatedEntityType, err := specGroup.TypeGenerator(groupLabel, entityID, config.Groups, config.ClusterName)
				if err != nil {
					errs = append(errs, fmt.Errorf("error generating entity type for %s: %s", entityID, err))
					continue
				}

				// 3. Loop through the created sub-groups and create a unique entity for each.
				for _, subGroupMetrics := range subGroups {
					// This is the new, unique ID for our sub-entity.
					//subEntityID := fmt.Sprintf("%s_%s", entityID, subGroupKey)
					subEntityID := entityID

					// Create the entity directly with the new ID and the generated type.
					e, err := config.Integration.Entity(subEntityID, generatedEntityType)
					if err != nil {
						errs = append(errs, err)
						continue
					}

					// Add standard attributes.
					extraAttributes := []attribute.Attribute{
						attribute.Attr("clusterName", config.ClusterName),
						attribute.Attr("displayName", e.Metadata.Name),
					}
					e.AddAttributes(extraAttributes...)

					// Create the metric set.
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

					// Create a temporary, scoped RawGroups map for this sub-entity.
					groupsForThisSubEntity := definition.RawGroups{
						groupLabel: {
							subEntityID: subGroupMetrics,
						},
					}

					// Call metricSetPopulate with the correctly scoped data.
					wasPopulated, populateErrs := metricSetPopulate(ms, groupLabel, subEntityID, groupsForThisSubEntity, config.Specs)
					if len(populateErrs) > 0 {
						errs = append(errs, populateErrs...)
					}
					if wasPopulated {
						populated = true
					}
				}

				// We're done with this group, continue to the next one.
				continue
			}

			// DEBUG: Announce that we are taking the default path.
			fmt.Printf(">>> Populating SINGLE ENTITY with ID: %s\n", entityID)

			singlePopulated, singleErrs := populateSingleEntity(config, groupLabel, entityID, rawMetrics, config.Groups)
			if len(singleErrs) > 0 {
				errs = append(errs, singleErrs...)
			}
			if singlePopulated {
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

// populateSingleEntity creates and populates a single entity in the integration.
// It correctly scopes the data passed to the metric population step.
func populateSingleEntity(config *definition.IntegrationPopulateConfig, groupLabel, entityID string, metrics definition.RawMetrics, allGroups definition.RawGroups) (bool, []error) {
	var errs []error
	var msEntityType string

	// ID and Type Generators use the full 'allGroups' map for context if needed.
	msEntityID := entityID
	if generator := config.Specs[groupLabel].IDGenerator; generator != nil {
		generatedEntityID, err := generator(groupLabel, entityID, allGroups)
		if err != nil {
			return false, []error{fmt.Errorf("error generating entity ID for %s: %s", entityID, err)}
		}
		msEntityID = generatedEntityID
	}

	if generatorType := config.Specs[groupLabel].TypeGenerator; generatorType != nil {
		generatedEntityType, err := generatorType(groupLabel, entityID, allGroups, config.ClusterName)
		if err != nil {
			return false, []error{fmt.Errorf("error generating entity type for %s: %s", entityID, err)}
		}
		msEntityType = generatedEntityType
	}

	// Create the entity in the integration payload.
	e, err := config.Integration.Entity(msEntityID, msEntityType)
	if err != nil {
		return false, []error{err}
	}

	// Add common attributes.
	extraAttributes := []attribute.Attribute{
		attribute.Attr("clusterName", config.ClusterName),
		attribute.Attr("displayName", e.Metadata.Name),
	}
	e.AddAttributes(extraAttributes...)

	// Create the MetricSet for this entity.
	msTypeGuesser := config.MsTypeGuesser
	if customGuesser := config.Specs[groupLabel].MsTypeGuesser; customGuesser != nil {
		msTypeGuesser = customGuesser
	}
	msType, err := msTypeGuesser(groupLabel)
	if err != nil {
		return false, []error{err}
	}

	ms := e.NewMetricSet(msType)

	// Create a temporary, scoped RawGroups map containing only the metrics for this specific entity.
	groupsForThisEntity := definition.RawGroups{
		groupLabel: {
			entityID: metrics,
		},
	}

	// Pass the correctly scoped map to metricSetPopulate.
	wasPopulated, populateErrs := metricSetPopulate(ms, groupLabel, entityID, groupsForThisEntity, config.Specs)
	if len(populateErrs) > 0 {
		for _, err := range populateErrs {
			errs = append(errs, fmt.Errorf("error populating metric for entity ID %s: %s", entityID, err))
		}
	}

	return wasPopulated, errs
}

func metricSetPopulate(ms *metric.Set, groupLabel, entityID string, groups definition.RawGroups, specs definition.SpecGroups) (populated bool, errs []error) {
	// DEBUG: Print the entity this metric set belongs to.
	fmt.Printf("--- Populating metrics for Entity: %s ---\n", entityID)

	for _, ex := range specs[groupLabel].Specs {
		val, err := ex.ValueFunc(groupLabel, entityID, groups)
		if err != nil {
			if !ex.Optional {
				fmt.Printf("  [ERROR] Cannot fetch value for metric %q: %v\n", ex.Name, err)
				errs = append(errs, fmt.Errorf("cannot fetch value for metric %q: %w", ex.Name, err))
			}
			continue
		}

		if val == nil {
			continue // Skip nil values
		}

		if typedMultiple, ok := val.(definition.FetchedTypedValues); ok {
			for k, typedV := range typedMultiple {
				// Use the type from the value (typedV.Type) instead of the spec (ex.Type).
				err := ms.SetMetric(k, typedV.Value, typedV.Type)
				if err != nil { /* ... error handling ... */
					errs = append(errs, fmt.Errorf("cannot set metric %s with value %v in metric set, %s", k, typedV.Value, err))
				}
				populated = true
			}
			// Fallback to the original logic for older ValueFuncs.
		} else if multiple, ok := val.(definition.FetchedValues); ok {
			for k, v := range multiple {
				// DEBUG: Print each key-value pair from a multi-value fetcher.
				fmt.Printf("  [Metric] Key: %-30s | Value: %v\n", k, v)

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
			// DEBUG: Print the key-value pair for a single-value fetcher.
			fmt.Printf("  [Metric] Key: %-30s | Value: %v\n", ex.Name, val)

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
