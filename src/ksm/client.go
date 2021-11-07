package ksm

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"

// ksm implements Client interface
type ksm struct {
	client doer
	logger log.Logger
}

// Doer is the interface that ksm client should satisfy.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewKSMClient(logger log.Logger) (Client, error) {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 3
	c.Timeout = 10 * time.Second
	c.LogHook = func(e pester.ErrEntry) {
		logger.Debugf("getting data from ksm: %q", e)
	}

	if logger == nil {
		return nil, fmt.Errorf("logger not provided")
	}

	return &ksm{
		client: c,
		logger: logger,
	}, nil
}

type Client interface {
	MetricFamiliesGetterForEndpoint(endpoint string, schema string) prometheus.FilteredMetricFamilies
}

func (c *ksm) MetricFamiliesGetterForEndpoint(endpoint string, schema string) prometheus.FilteredMetricFamilies {
	ksmMetricsURL := url.URL{
		Scheme: schema,
		Host:   endpoint,
		Path:   metric.PrometheusMetricsPath,
	}

	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c, ksmMetricsURL.String(), queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}

func (c *ksm) Do(r *http.Request) (*http.Response, error) {
	c.logger.Debugf("Calling kube-state-metrics endpoint: %s", r.URL.String())

	// Calls http.Client.
	return c.client.Do(r)
}

func NewEndpointsDiscoverer(client kubernetes.Interface, opts ...EndpointDiscoveryOptions) (discovery.EndpointsDiscoverer, error) {
	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute
	stopCh := make(chan struct{})
	discoveryConfig := discovery.EndpointsDiscoveryConfig{
		LabelSelector: defaultLabelSelector,

		EndpointsLister: func(options ...informers.SharedInformerOption) discovery.EndpointsLister {
			factory := informers.NewSharedInformerFactoryWithOptions(client, resyncDuration, options...)

			lister := factory.Core().V1().Endpoints().Lister()

			factory.Start(stopCh)
			factory.WaitForCacheSync(stopCh)

			return lister
		},
	}

	for _, optFunc := range opts {
		err := optFunc(&discoveryConfig)
		if err != nil {
			return nil, err
		}
	}

	return discovery.NewEndpointsDiscoverer(discoveryConfig)
}

type EndpointDiscoveryOptions func(*discovery.EndpointsDiscoveryConfig) error

func WithNamespace(ns string) EndpointDiscoveryOptions {
	return func(edc *discovery.EndpointsDiscoveryConfig) error {
		edc.Namespace = ns

		return nil
	}
}

func WithLabelSelector(label string) EndpointDiscoveryOptions {
	return func(edc *discovery.EndpointsDiscoveryConfig) error {
		edc.LabelSelector = label

		return nil
	}
}

func WithPort(port int) EndpointDiscoveryOptions {
	return func(edc *discovery.EndpointsDiscoveryConfig) error {
		edc.Port = port

		return nil
	}
}

func WithFixedEndpoint(fixedEndpoint string) EndpointDiscoveryOptions {
	return func(edc *discovery.EndpointsDiscoveryConfig) error {
		edc.FixedEndpoint = []string{fixedEndpoint}

		return nil
	}
}
