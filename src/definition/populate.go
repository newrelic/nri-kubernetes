package definition

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
)

// GuessFunc guesses from data.
type GuessFunc func(clusterName, groupLabel, entityID string, groups RawGroups) (string, error)

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

	return ms.SetMetric("clusterK8sVersion", k8sVersionStr, metric.ATTRIBUTE)
}

type IntegrationPopulateConfig struct {
	Integration   *integration.Integration
	ClusterName   string
	K8sVersion    fmt.Stringer
	MsTypeGuesser GuessFunc
	Groups        RawGroups
	Specs         SpecGroups
}

// IntegrationPopulator populates an integration with the given metrics and definition.
func IntegrationPopulator(config *IntegrationPopulateConfig) (bool, []error) {
	var populated bool
	var errs []error
	var msEntityType string
	for groupLabel, entities := range config.Groups {
		for entityID := range entities {

			// Only populate specified groups.
			if _, ok := config.Specs[groupLabel]; !ok {
				continue
			}

			msEntityID := entityID
			if generator := config.Specs[groupLabel].IDGenerator; generator != nil {
				generatedEntityID, err := generator(groupLabel, entityID, config.Groups)
				if err != nil {
					errs = append(errs, fmt.Errorf("error generating entity ID for %s: %s", entityID, err))
					continue
				}
				msEntityID = generatedEntityID
			}

			if generatorType := config.Specs[groupLabel].TypeGenerator; generatorType != nil {
				generatedEntityType, err := generatorType(groupLabel, entityID, config.Groups, config.ClusterName)
				if err != nil {
					errs = append(errs, fmt.Errorf("error generating entity type for %s: %s", entityID, err))
					continue
				}
				msEntityType = generatedEntityType
			}

			e, err := config.Integration.Entity(msEntityID, msEntityType)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			// Add entity attributes, which will propagate to all metric.Sets.
			// This was previously (on sdk v2) done by msManipulators.
			e.AddAttributes(
				attribute.Attr("clusterName", config.ClusterName),
				attribute.Attr("displayName", e.Metadata.Name),
			)

			msType, err := config.MsTypeGuesser(config.ClusterName, groupLabel, entityID, config.Groups)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			ms := e.NewMetricSet(msType)

			wasPopulated, populateErrs := metricSetPopulate(ms, groupLabel, entityID, config.Groups, config.Specs)
			if len(populateErrs) != 0 {
				for _, err := range populateErrs {
					errs = append(errs, fmt.Errorf("error populating metric for entity ID %s: %s", entityID, err))
				}
			}

			if wasPopulated {
				populated = true
			}
		}
	}
	if populated {
		err := populateCluster(config.Integration, config.ClusterName, config.K8sVersion)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return populated, errs
}

func metricSetPopulate(ms *metric.Set, groupLabel, entityID string, groups RawGroups, specs SpecGroups) (populated bool, errs []error) {
	for _, ex := range specs[groupLabel].Specs {
		val, err := ex.ValueFunc(groupLabel, entityID, groups)
		if err != nil {
			if !ex.Optional {
				errs = append(errs, fmt.Errorf("cannot fetch value for metric %q: %w", ex.Name, err))
			}
			continue
		}

		if multiple, ok := val.(FetchedValues); ok {
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
