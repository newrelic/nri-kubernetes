package grouper

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

type grouper struct {
	Config
	logger *log.Logger
}

type Config struct {
	Queries                    []prometheus.Query
	MetricFamiliesGetter       prometheus.FetchAndFilterMetricsFamilies
	ServicesLister             listersv1.ServiceLister
	EnableResourceQuotaSamples bool
}

type OptionFunc func(kc *grouper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kc *grouper) error {
		kc.logger = logger
		return nil
	}
}

// New returns a data.Grouper that groups KSM metrics.
func New(config Config, opts ...OptionFunc) (data.Grouper, error) {
	if config.MetricFamiliesGetter == nil {
		return nil, fmt.Errorf("metric families getter must be set")
	}

	if config.ServicesLister == nil {
		return nil, fmt.Errorf("ServicesLister must be set")
	}

	g := &grouper{
		Config: config,
		logger: logutil.Discard,
	}

	for i, opt := range opts {
		if err := opt(g); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return g, nil
}

// Group implements Grouper interface by fetching Prometheus metrics from KSM and then modifying it
// using Service objects fetched from API server.
func (g *grouper) Group(specGroups definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	mFamily, err := g.MetricFamiliesGetter(g.Queries)
	if err != nil {
		return nil, &data.ErrorGroup{
			Errors: []error{fmt.Errorf("querying KSM: %w", err)},
		}
	}

	groups, errs := prometheus.GroupMetricsBySpec(specGroups, mFamily)
	if servicesGroup, ok := groups["service"]; ok {
		if err := g.addServiceSpecSelectorToGroup(servicesGroup); err != nil {
			errs = append(errs, fmt.Errorf("adding service spec selector to group: %w", err))
		}
	}

	if !g.EnableResourceQuotaSamples {
		if _, ok := groups["resourcequota"]; ok {
			delete(groups, "resourcequota")
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
func (g *grouper) addServiceSpecSelectorToGroup(serviceGroup map[string]definition.RawMetrics) error {
	services, err := g.ServicesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}

	for _, s := range services {
		serviceRawMetrics, ok := serviceGroup[fmt.Sprintf("%s_%s", s.Namespace, s.Name)]
		if !ok {
			g.logger.Debugf("Metrics for service %s.%s not found in cluster", s.Namespace, s.Name)
			continue
		}

		promLabels := prometheus.Labels{}

		for key, value := range s.Spec.Selector {
			promLabels[fmt.Sprintf("selector_%s", key)] = value
		}

		serviceRawMetrics["apiserver_kube_service_spec_selectors"] = prometheus.Metric{
			Labels: promLabels,
			Value:  nil,
		}
	}
	return nil
}
