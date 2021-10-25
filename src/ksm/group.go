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
	client                 client.HTTPGetter
	queries                []prometheus.Query
	filteredMetricFamilies prometheus.FilteredMetricFamilies
	logger                 log.Logger
	k8sClient              client.Kubernetes
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

// Group implements Grouper interface by fetching Prometheus metrics from KSM and then modifying it
// using Service objects fetched from API server.
func (r *ksmGrouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	getter := func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		return prometheus.Do(r.client, metric.PrometheusMetricsPath, r.queries)
	}

	if r.filteredMetricFamilies != nil {
		getter = r.filteredMetricFamilies
	}

	mFamily, err := getter(r.queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{fmt.Errorf("querying KSM: %w", err)},
		}
	}

	groups, errs := prometheus.GroupMetricsBySpec(specGroups, mFamily)
	if servicesGroup, ok := groups["service"]; ok {
		if err := r.addServiceSpecSelectorToGroup(servicesGroup); err != nil {
			errs = append(errs, fmt.Errorf("adding service spec selector to group: %w", err))
		}
	}

	if len(errs) > 0 {
		return groups, &data.ErrorGroup{
			Recoverable: true,
			Errors:      errs,
		}
	}

	return groups, nil
}

// NewGrouper creates a grouper aware of Kube State Metrics raw metrics.
func NewGrouper(c client.HTTPGetter, queries []prometheus.Query, logger log.Logger, k8sClient client.Kubernetes) data.Grouper {
	return &ksmGrouper{
		queries:   queries,
		client:    c,
		logger:    logger,
		k8sClient: k8sClient,
	}
}

type GrouperConfig struct {
	Queries              []prometheus.Query
	MetricFamiliesGetter prometheus.FilteredMetricFamilies
	Logger               log.Logger
	K8sClient            client.Kubernetes
}

func NewValidatedGrouper(config *GrouperConfig) (data.Grouper, error) {
	if config == nil {
		return nil, fmt.Errorf("config must be provided")
	}

	if config.MetricFamiliesGetter == nil {
		return nil, fmt.Errorf("metric families getter must be set")
	}

	if config.Logger == nil {
		return nil, fmt.Errorf("logger must be set")
	}

	if config.K8sClient == nil {
		return nil, fmt.Errorf("k8s client must be set")
	}

	return &ksmGrouper{
		queries:                config.Queries,
		filteredMetricFamilies: config.MetricFamiliesGetter,
		logger:                 config.Logger,
		k8sClient:              config.K8sClient,
	}, nil
}
