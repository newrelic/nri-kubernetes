package ksm

import (
	"fmt"
	"io"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	k8sClient "github.com/newrelic/nri-kubernetes/v2/src/client"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	ksmGrouper "github.com/newrelic/nri-kubernetes/v2/src/ksm/grouper"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
)

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"

// Providers is a struct holding pointers to all the clients Scraper needs to get data from
// TODO: Extract this out of the KSM package
type Providers struct {
	K8s k8sClient.Kubernetes
	KSM ksmClient.Client
}

// Scraper takes care of getting metrics from an autodiscovered KSM instance
type Scraper struct {
	logger log.Logger
	config *config.Mock
	Providers
	k8sVersion          *version.Info
	endpointsDiscoverer discovery.EndpointsDiscoverer
	servicesLister      discovery.ServicesLister
}

type ScraperOpt func(s *Scraper) error

func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

func NewScraper(config *config.Mock, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	s := &Scraper{
		config:    config,
		Providers: providers,
		// TODO: An empty implementation of the logger interface would be better
		logger: log.New(false, io.Discard),
	}

	// TODO: Sanity check config
	// return nil, ConfigErr...

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

	k8sVersion, err := providers.K8s.GetClient().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	// Assume Kubernetes version will not change during the lifetime of the integration, and store it
	s.k8sVersion = k8sVersion

	endpointsDiscoverer, err := s.buildDiscoverer()
	if err != nil {
		return nil, fmt.Errorf("building endpoints disoverer: %w", err)
	}

	s.endpointsDiscoverer = endpointsDiscoverer

	// Discard stop channel since we will run forever
	// TODO: Expose a Stop() method?
	s.servicesLister, _ = discovery.NewServicesLister(providers.K8s.GetClient())

	return s, nil
}

func (s *Scraper) Run(i *integration.Integration) error {
	populated := false

	endpoints, err := s.endpointsDiscoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering KSM endpoints: %w", err)
	}

	s.logger.Debugf("Discovered endpoints: %q", endpoints)

	services, err := s.servicesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("discovering KSM services: %w", err)
	}

	for _, endpoint := range endpoints {
		ksmGrouperConfig := &ksmGrouper.Config{
			MetricFamiliesGetter: s.KSM.MetricFamiliesGetterForEndpoint(endpoint, s.config.KSM.Scheme),
			Logger:               s.logger,
			Services:             services,
			Queries:              metric.KSMQueries,
		}

		ksmGrouper, err := ksmGrouper.NewValidatedGrouper(ksmGrouperConfig)
		if err != nil {
			return fmt.Errorf("creating KSM grouper: %w", err)
		}

		// TODO: Check if the concept of job still makes sense with the new architecture
		job := scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs)

		s.logger.Debugf("Running job: %s", job.Name)

		r := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)
		if r.Errors != nil {
			s.logger.Debugf("populating KMS: %v", r.Error())
		}

		if r.Populated && !s.config.KSM.Distributed {
			populated = true
			break
		}
	}

	if !populated {
		return fmt.Errorf("KSM data was not populated after trying all endpoints")
	}

	return nil
}

func (s *Scraper) buildDiscoverer() (discovery.EndpointsDiscoverer, error) {
	dc := discovery.EndpointsDiscoveryConfig{
		LabelSelector: defaultLabelSelector,
		Client:        s.K8s.GetClient(),
	}

	if s.config.KSM.Host != "" {
		s.logger.Debugf("ksm discovery disabled")
		dc.FixedEndpoint = []string{s.config.KSM.Host}
	}

	if s.config.KSM.Namespace != "" {
		dc.Namespace = s.config.KSM.Namespace
	}

	if s.config.KSM.PodLabel != "" {
		dc.LabelSelector = s.config.KSM.PodLabel
	}

	if s.config.KSM.Port != 0 {
		dc.Port = s.config.KSM.Port
	}

	return discovery.NewEndpointsDiscoverer(dc)
}
