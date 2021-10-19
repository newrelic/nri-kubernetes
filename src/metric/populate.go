package metric

import (
	"errors"
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

// MultipleErrs represents a bunch of errs.
// Recoverable == true means that you can keep working with those errors.
// Recoverable == false means you must handle the errors or panic.
type MultipleErrs struct {
	Errs []error
}

// Error implements error interface
func (e MultipleErrs) Error() string {
	s := "multiple errors:"

	for _, err := range e.Errs {
		s = fmt.Sprintf("%s\n%s", s, err)
	}
	return s
}

// Populate populates k8s raw data to sdk metrics.
func Populate(
	groups definition.RawGroups,
	specGroups definition.SpecGroups,
	i *integration.Integration,
	clusterName string,
	k8sVersion *version.Info,
) data.PopulateResult {
	populatorFunc := definition.IntegrationPopulator(i, clusterName, k8sVersion, K8sMetricSetTypeGuesser)
	ok, errs := populatorFunc(groups, specGroups)

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
