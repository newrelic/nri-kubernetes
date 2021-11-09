package grouper

import (
	"fmt"

	"github.com/newrelic/infra-integrations-sdk/log"
	corev1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type ksmGrouper struct {
	queries                []prometheus.Query
	filteredMetricFamilies prometheus.FilteredMetricFamilies
	logger                 log.Logger
	services               []*corev1.Service
}

type Config struct {
	Queries              []prometheus.Query
	MetricFamiliesGetter prometheus.FilteredMetricFamilies
	Logger               log.Logger
	Services             []*corev1.Service
}

func NewValidatedGrouper(config *Config) (data.Grouper, error) {
	if config == nil {
		return nil, fmt.Errorf("config must be provided")
	}

	if config.MetricFamiliesGetter == nil {
		return nil, fmt.Errorf("metric families getter must be set")
	}

	if config.Logger == nil {
		return nil, fmt.Errorf("logger must be set")
	}

	return &ksmGrouper{
		queries:                config.Queries,
		filteredMetricFamilies: config.MetricFamiliesGetter,
		logger:                 config.Logger,
	}, nil
}

// Group implements Grouper interface by fetching Prometheus metrics from KSM and then modifying it
// using Service objects fetched from API server.
func (r *ksmGrouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {

	mFamily, err := r.filteredMetricFamilies(r.queries)
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

// addServiceSpecSelectorToGroup adds a new metric to the service group
// which includes the selectors defined in the service spec.
func (r *ksmGrouper) addServiceSpecSelectorToGroup(serviceGroup map[string]definition.RawMetrics) error {
	for _, s := range r.services {
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
