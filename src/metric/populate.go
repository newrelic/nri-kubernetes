package metric

import (
	"errors"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

// Populate populates k8s raw data to sdk metrics.
func Populate(
	groups definition.RawGroups,
	specGroups definition.SpecGroups,
	i *integration.Integration,
	clusterName string,
	k8sVersion *version.Info,
) data.PopulateResult {
	ok, errs := definition.IntegrationPopulator(i, clusterName, k8sVersion, K8sMetricSetTypeGuesser, groups, specGroups)

	if len(errs) > 0 {
		return data.PopulateResult{Errors: errs, Populated: ok}
	}

	// This should not happen ideally if no errors were reported.
	if !ok {
		return data.PopulateResult{
			Errors:    []error{errors.New("no data was populated")},
			Populated: false,
		}
	}

	return data.PopulateResult{Errors: nil, Populated: true}
}
