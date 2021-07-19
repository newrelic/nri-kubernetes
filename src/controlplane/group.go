package controlplane

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// prometheusMetricsPath is the control plane component prometheus
// metrics endpoint
const prometheusMetricsPath = "/metrics"

type componentGrouper struct {
	queries []prometheus.Query
	client  client.HTTPClient
	logger  log.Logger
	podName string
}

func (r *componentGrouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	mFamily, err := prometheus.Do(r.client, prometheusMetricsPath, r.queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Recoverable: false,
			Errors: []error{
				fmt.Errorf("error querying controlplane component %s: %s", r.podName, err),
			},
		}
	}

	groups, errs := prometheus.GroupEntityMetricsBySpec(specGroups, mFamily, r.podName)
	if len(errs) > 0 {
		return groups, &data.ErrorGroup{Recoverable: true, Errors: errs}
	}
	return groups, nil
}

// NewComponentGrouper creates a grouper for the given control plane
// component podName.
func NewComponentGrouper(
	c client.HTTPClient,
	queries []prometheus.Query,
	logger log.Logger,
	podName string,
) data.Grouper {
	return &componentGrouper{
		queries: queries,
		client:  c,
		logger:  logger,
		podName: podName,
	}
}
