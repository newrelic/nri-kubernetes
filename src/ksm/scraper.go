package ksm

import (
	"fmt"
	"io"
	"net"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	ksmGrouper "github.com/newrelic/nri-kubernetes/v2/src/ksm/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
)

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"
const defaultScheme = "http"
const ksmMetricsPath = "metrics"

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the KSM package.
type Providers struct {
	K8s kubernetes.Interface
	KSM ksmClient.MetricFamiliesGetter
}

// Scraper takes care of getting metrics from an autodiscovered KSM instance.
type Scraper struct {
	logger      log.Logger
	clusterName string
	config      config.KSM
	Providers
	k8sVersion          *version.Info
	endpointsDiscoverer discovery.EndpointsDiscoverer
	servicesLister      discovery.ServicesLister
	informerClosers     []chan<- struct{}
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
// Close() to prevent resource leakage.
func NewScraper(clusterName string, cfg config.KSM, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	s := &Scraper{
		clusterName: clusterName,
		config:      cfg,
		Providers:   providers,
		// TODO: An empty implementation of the logger interface would be better
		logger: log.New(false, io.Discard),
	}

	// TODO: Sanity check config
	if s.config.Discovery.Scheme == "" {
		s.config.Discovery.Scheme = config.DefaultSchema
	}

	if u, err := url.Parse(s.config.Discovery.Static.URL); err != nil {
		s.config.Discovery.Static.URL = net.JoinHostPort(u.Host, u.Port())
		s.config.Discovery.Scheme = u.Scheme
	}

	// return nil, ConfigErr...

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

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
			MetricFamiliesGetter: s.KSM.MetricFamiliesGetter(endpoint),
			Queries:              metric.KSMQueries,
			ServicesLister:       s.servicesLister,
		}, ksmGrouper.WithLogger(s.logger))
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		// TODO: Check if the concept of job still makes sense with the new architecture.
		job := scrape.NewScrapeJob("kube-state-metrics", grouper, metric.KSMSpecs)

		s.logger.Debugf("Running KSM job")
		r := job.Populate(i, s.clusterName, s.logger, s.k8sVersion)
		if r.Errors != nil {
			s.logger.Warnf("Error populating KSM metrics: %v", r.Error())
		}

		if !r.Populated {
			log.Debug("No metrics were populated, trying next endpoint")
			continue
		}

		populated = r.Populated

		if !s.config.Discovery.Distributed {
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

	if s.config.Discovery.Endpoints.Namespace != "" {
		s.logger.Debugf("Restricting KSM discovery to namespace %q", s.config.Discovery.Endpoints.Namespace)
		dc.Namespace = s.config.Discovery.Endpoints.Namespace
	}

	if s.config.Discovery.Endpoints.LabelSelector != "" {
		s.logger.Debugf("Overriding default KSM labelSelector (%q) to %q", defaultLabelSelector, s.config.Discovery.Endpoints.LabelSelector)
		dc.LabelSelector = s.config.Discovery.Endpoints.LabelSelector
	}

	if s.config.Discovery.Port != 0 {
		s.logger.Debugf("Overriding default KSM port to %d", defaultLabelSelector, s.config.Discovery.Port)
		dc.Port = s.config.Discovery.Port
	}

	return discovery.NewEndpointsDiscoverer(dc)
}

func (s *Scraper) ksmURLs() ([]string, error) {
	if u := s.config.Discovery.Static.URL; u != "" {
		s.logger.Debugf("Using overridden endpoint for ksm %q", u)
		return []string{u}, nil
	}

	endpoints, err := s.endpointsDiscoverer.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	scheme := s.config.Discovery.Scheme
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
