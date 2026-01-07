package ksm

import (
	"fmt"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	ksmGrouper "github.com/newrelic/nri-kubernetes/v3/src/ksm/grouper"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/newrelic/nri-kubernetes/v3/src/scrape"
)

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"
const defaultScheme = "http"
const ksmMetricsPath = "metrics"

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the KSM package.
type Providers struct {
	K8s kubernetes.Interface
	KSM prometheus.MetricFamiliesGetFunc
}

// Scraper takes care of getting metrics from an autodiscovered KSM instance.
type Scraper struct {
	Providers
	logger                   *log.Logger
	config                   *config.Config
	k8sVersion               *version.Info
	endpointsDiscoverer      discovery.EndpointsDiscoverer
	endpointSlicesDiscoverer discovery.EndpointSlicesDiscoverer
	servicesLister           listersv1.ServiceLister
	informerClosers          []chan<- struct{}
	Filterer                 discovery.NamespaceFilterer
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

// WithFilterer returns an OptionFunc to add a Filterer.
func WithFilterer(filterer discovery.NamespaceFilterer) ScraperOpt {
	return func(s *Scraper) error {
		s.Filterer = filterer
		return nil
	}
}

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
// Close() to prevent resource leakage.
func NewScraper(config *config.Config, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	s := &Scraper{
		config:    config,
		Providers: providers,
		logger:    logutil.Discard,
	}

	// TODO: Sanity check config
	// return nil, ConfigErr...

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig
	k8sVersion, err := providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}
	s.logger.Debugf("Identified cluster version: %s", k8sVersion)

	// Assume Kubernetes version will not change during the lifetime of the integration, and store it
	s.k8sVersion = k8sVersion

	s.logger.Debugf("Building KSM discoverer")
	endpointsDiscoverer, err := s.buildDiscoverer()
	if err != nil {
		return nil, fmt.Errorf("building endpoints disoverer: %w", err)
	}

	s.endpointsDiscoverer = endpointsDiscoverer

	s.logger.Debugf("Building KSM endpoint slices discoverer")
	endpointSlicesDiscoverer, err := s.buildEndpointSlicesDiscoverer()
	if err != nil {
		return nil, fmt.Errorf("building endpoint slices disoverer: %w", err)
	}
	s.endpointSlicesDiscoverer = endpointSlicesDiscoverer

	servicesLister, servicesCloser := discovery.NewServicesLister(providers.K8s)
	s.servicesLister = servicesLister
	s.informerClosers = append(s.informerClosers, servicesCloser)

	return s, nil
}

// Run runs the scraper, adding all the KSM-related metrics and entities into the integration i.
// Run must not be called after Close().
func (s *Scraper) Run(i *integration.Integration) error {
	populated := false

	endpoints, err := s.ksmURLs()
	if err != nil {
		return err
	}

	s.logger.Debugf("Discovered endpoints: %q", endpoints)

	for _, endpoint := range endpoints {
		s.logger.Debugf("Fetching KSM data from %q", endpoint)
		grouper, err := ksmGrouper.New(ksmGrouper.Config{
			MetricFamiliesGetter:       s.KSM.MetricFamiliesGetFunc(endpoint),
			Queries:                    metric.KSMQueries,
			ServicesLister:             s.servicesLister,
			EnableResourceQuotaSamples: s.config.EnableResourceQuotaSamples,
		}, ksmGrouper.WithLogger(s.logger))
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		// TODO: Check if the concept of job still makes sense with the new architecture.
		job := scrape.NewScrapeJob("kube-state-metrics", grouper, metric.KSMSpecs, scrape.JobWithFilterer(s.Filterer))

		s.logger.Debugf("Running KSM job")
		r := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)
		if r.Errors != nil {
			if r.Populated {
				s.logger.Tracef("Error populating KSM metrics: %v", r.Error())
			} else {
				s.logger.Warnf("Error populating KSM metrics: %v", r.Error())
			}
		}

		if !r.Populated {
			log.Debug("No metrics were populated, trying next endpoint")
			continue
		}

		populated = r.Populated

		if !s.config.KSM.Distributed {
			break
		}
	}

	if !populated {
		return fmt.Errorf("KSM data was not populated after trying all endpoints")
	}

	return nil
}

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}

// buildDiscoverer returns a discovery.EndpointsDiscoverer, configured to discover KSM endpoints in the cluster,
// or to return the static endpoint defined by the user in the config.
func (s *Scraper) buildDiscoverer() (discovery.EndpointsDiscoverer, error) {
	dc := discovery.EndpointsDiscoveryConfig{
		LabelSelector: defaultLabelSelector,
		Client:        s.K8s,
	}

	if s.config.KSM.Namespace != "" {
		s.logger.Debugf("Restricting KSM discovery to namespace %q", s.config.KSM.Namespace)
		dc.Namespace = s.config.KSM.Namespace
	}

	if s.config.KSM.Selector != "" {
		s.logger.Debugf("Overriding default KSM labelSelector (%q) to %q", defaultLabelSelector, s.config.KSM.Selector)
		dc.LabelSelector = s.config.KSM.Selector
	}

	if s.config.KSM.Port != 0 {
		s.logger.Debugf("Overriding default KSM port to %d", s.config.KSM.Port)
		dc.Port = s.config.KSM.Port
	}

	discoverer, err := discovery.NewEndpointsDiscoverer(dc)
	if err != nil {
		return nil, err
	}

	return &discovery.EndpointsDiscovererWithTimeout{
		EndpointsDiscoverer: discoverer,

		BackoffDelay: s.config.KSM.Discovery.BackoffDelay,
		Timeout:      s.config.KSM.Discovery.Timeout,
	}, nil
}

// buildEndpointSlicesDiscoverer returns a discovery.EndpointSlicesDiscoverer, configured to discover KSM endpoints in the cluster,
// or to return the static endpoint defined by the user in the config.
func (s *Scraper) buildEndpointSlicesDiscoverer() (discovery.EndpointSlicesDiscoverer, error) {
	dc := discovery.EndpointSlicesDiscoveryConfig{
		LabelSelector: defaultLabelSelector,
		Client:        s.K8s,
	}

	if s.config.KSM.Namespace != "" {
		s.logger.Debugf("Restricting KSM discovery to namespace %q", s.config.KSM.Namespace)
		dc.Namespace = s.config.KSM.Namespace
	}

	if s.config.KSM.Selector != "" {
		s.logger.Debugf("Overriding default KSM labelSelector (%q) to %q", defaultLabelSelector, s.config.KSM.Selector)
		dc.LabelSelector = s.config.KSM.Selector
	}

	if s.config.KSM.Port != 0 {
		s.logger.Debugf("Overriding default KSM port to %d", s.config.KSM.Port)
		dc.Port = s.config.KSM.Port
	}

	discoverer, err := discovery.NewEndpointSlicesDiscoverer(dc)
	if err != nil {
		return nil, err
	}

	return &discovery.EndpointSlicesDiscovererWithTimeout{
		EndpointsDiscoverer: discoverer,

		BackoffDelay: s.config.KSM.Discovery.BackoffDelay,
		Timeout:      s.config.KSM.Discovery.Timeout,
	}, nil
}

func (s *Scraper) ksmURLs() ([]string, error) {
	if u := s.config.KSM.StaticURL; u != "" {
		s.logger.Debugf("Using overridden endpoint for ksm %q", u)
		return []string{u}, nil
	}

	var endpoints []string
	var err error
	endpoints, err = s.endpointSlicesDiscoverer.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	//@todo: decide if we need this
	if false {
		endpoints, err = s.endpointsDiscoverer.Discover()
		if err != nil {
			return nil, fmt.Errorf("discovering KSM endpoints: %w", err)
		}
	}

	scheme := s.config.KSM.Scheme
	if scheme == "" {
		scheme = defaultScheme
	}

	urls := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		urls = append(urls, (&url.URL{
			Scheme: scheme,
			Host:   endpoint,
			Path:   ksmMetricsPath,
		}).String())
	}

	return urls, nil
}
