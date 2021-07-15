package ksm

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type ksmGrouper struct {
	queries   []prometheus.Query
	client    client.HTTPClient
	logger    log.Logger
	k8sClient client.Kubernetes
}

// addServiceSpecSelectorToGroup adds a new metric to the service group
// which includes the selectors defined in the service spec.
func (r *ksmGrouper) addServiceSpecSelectorToGroup(serviceGroup map[string]definition.RawMetrics) error {
	services, err := r.k8sClient.ListServices("")
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}
	for _, s := range services.Items {
		serviceRawMetrics, ok := serviceGroup[fmt.Sprintf("%s_%s", s.Namespace, s.Name)]
		if !ok {
			continue
		}

		labels := prometheus.Labels{}

		for key, value := range s.Spec.Selector {
			labels[fmt.Sprintf("selector_%s", key)] = value
		}

		serviceRawMetrics["apiserver_kube_service_spec_selectors"] = prometheus.Metric{
			Labels: labels,
			Value:  nil,
		}
	}
	return nil
}

func (r *ksmGrouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	mFamily, err := prometheus.Do(r.client, metric.PrometheusMetricsPath, r.queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Recoverable: false,
			Errors:      []error{fmt.Errorf("querying KSM: %w", err)},
		}
	}

	groups, errs := prometheus.GroupMetricsBySpec(specGroups, mFamily)
	if servicesGroup, ok := groups["service"]; ok {
		if err := r.addServiceSpecSelectorToGroup(servicesGroup); err != nil {
			errs = append(errs, fmt.Errorf("adding service spec selector to group: %w", err))
		}
	}

	if len(errs) == 0 {
		return groups, nil
	}

	return groups, &data.ErrorGroup{Recoverable: true, Errors: errs}
}

// NewGrouper creates a grouper aware of Kube State Metrics raw metrics.
func NewGrouper(c client.HTTPClient, queries []prometheus.Query, logger log.Logger, k8sClient client.Kubernetes) data.Grouper {
	return &ksmGrouper{
		queries:   queries,
		client:    c,
		logger:    logger,
		k8sClient: k8sClient,
	}
}
