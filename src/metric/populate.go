package metric

import (
	"errors"

	"fmt"

	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
)

type k8sPopulator struct {
}

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
func (p *k8sPopulator) Populate(groups definition.RawGroups, specGroups definition.SpecGroups, i *sdk.IntegrationProtocol2, clusterName string) (err *data.PopulateErr) {
	populatorFunc := definition.IntegrationProtocol2PopulateFunc(i, clusterName, K8sMetricSetTypeGuesser, K8sEntityMetricsManipulator, K8sClusterMetricsManipulator)
	ok, errs := populatorFunc(groups, specGroups)

	if len(errs) > 0 {
		err = &data.PopulateErr{
			Errs:      errs,
			Populated: ok,
		}
		return
	}

	// This should not happen ideally if no errors were reported.
	if !ok {
		return &data.PopulateErr{
			Errs:      []error{errors.New("no data was populated")},
			Populated: ok,
		}
	}

	return err
}

// NewK8sPopulator creates a Kubernetes aware populator.
func NewK8sPopulator() data.Populator {
	return &k8sPopulator{}
}
