package ksm

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/ksm/metric"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/sirupsen/logrus"
)

type ksmGrouper struct {
	queries []prometheus.Query
	client  client.HTTPClient
	logger  *logrus.Logger
}

func (r *ksmGrouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	mFamily, err := prometheus.Do(r.client, metric.PrometheusMetricsPath, r.queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Recoverable: false,
			Errors:      []error{fmt.Errorf("error querying KSM. %s", err)},
		}
	}

	groups, errs := prometheus.GroupMetricsBySpec(specGroups, mFamily)
	if len(errs) == 0 {
		return groups, nil
	}
	return groups, &data.ErrorGroup{Recoverable: true, Errors: errs}
}

// NewGrouper creates a grouper aware of Kube State Metrics raw metrics.
func NewGrouper(c client.HTTPClient, queries []prometheus.Query, logger *logrus.Logger) data.Grouper {
	return &ksmGrouper{
		queries: queries,
		client:  c,
		logger:  logger,
	}
}
