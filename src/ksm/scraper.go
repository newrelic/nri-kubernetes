package ksm

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/ksm/crd"
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
	logger              *log.Logger
	config              *config.Config
	k8sVersion          *version.Info
	endpointsDiscoverer discovery.EndpointsDiscoverer
	servicesLister      listersv1.ServiceLister
	informerClosers     []chan<- struct{}
	Filterer            discovery.NamespaceFilterer
	crdHarvester        *telemetry.Harvester
	closeOnce           sync.Once
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

	servicesLister, servicesCloser := discovery.NewServicesLister(providers.K8s)
	s.servicesLister = servicesLister
	s.informerClosers = append(s.informerClosers, servicesCloser)

	// Initialize CRD metrics harvester if enabled
	if config.KSM.EnableCustomResourceMetrics {
		s.logger.Info("Initializing telemetry harvester for CRD dimensional metrics")

		// Read license key from environment (same as infrastructure agent uses)
		licenseKey := os.Getenv("NRIA_LICENSE_KEY")
		if licenseKey == "" {
			s.logger.Warn("NRIA_LICENSE_KEY not set - CRD dimensional metrics will not be sent")
		} else {
			// Log first 8 chars of license key for debugging
			maskedKey := licenseKey
			if len(licenseKey) > 8 {
				maskedKey = licenseKey[:8] + "..." + licenseKey[len(licenseKey)-4:]
			}
			s.logger.Infof("Using license key: %s", maskedKey)

			// Determine harvest period: use configured value or derive from scrape interval
			harvestPeriod := config.KSM.HarvestPeriod
			if harvestPeriod == 0 {
				// Default: match scrape interval for consistency with entity-based metrics
				harvestPeriod = config.Interval
				s.logger.Infof("CRD harvest period not configured, using scrape interval: %v", harvestPeriod)
			}

			// Create harvester to send dimensional metrics to New Relic Metric API
			harvestOpts := []func(*telemetry.Config){
				telemetry.ConfigAPIKey(licenseKey),
				telemetry.ConfigHarvestPeriod(harvestPeriod), // Automatic batching for better performance
			}
			s.logger.Infof("CRD metrics will be sent every %v (async batching)", harvestPeriod)

			// Use custom metric API URL if configured, otherwise default to production
			metricsURL := "https://metric-api.newrelic.com/metric/v1" // production default
			if config.KSM.MetricAPIURL != "" {
				metricsURL = config.KSM.MetricAPIURL
				s.logger.Infof("Using custom Metric API endpoint for CRD metrics: %s", metricsURL)
				harvestOpts = append(harvestOpts, telemetry.ConfigMetricsURLOverride(metricsURL))
			} else {
				s.logger.Infof("Using default Metric API endpoint for CRD metrics: %s", metricsURL)
			}

			// Add debug logger to see HTTP responses
			harvestOpts = append(harvestOpts, telemetry.ConfigBasicDebugLogger(s.logger.Writer()))

			harvester, err := telemetry.NewHarvester(harvestOpts...)
			if err != nil {
				return nil, fmt.Errorf("creating telemetry harvester for CRD metrics: %w", err)
			}
			s.crdHarvester = harvester
		}
	}

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

		// Build combined query list: standard KSM queries + CRD query if enabled
		queries := make([]prometheus.Query, 0, len(metric.KSMQueries)+1)
		queries = append(queries, metric.KSMQueries...)

		if s.config.KSM.EnableCustomResourceMetrics && s.crdHarvester != nil {
			// Add CRD prefix query to fetch all CRD metrics
			queries = append(queries, prometheus.Query{
				MetricName: "kube_customresource",
				Prefix:     true,
			})
		}

		// Fetch all metrics once (both standard and CRD)
		metricFamiliesGetter := s.KSM.MetricFamiliesGetFunc(endpoint)
		allMetricFamilies, err := metricFamiliesGetter(queries)
		if err != nil {
			s.logger.Warnf("Error fetching metrics from KSM: %v", err)
			continue
		}

		s.logger.Debugf("Fetched %d metric families from KSM", len(allMetricFamilies))

		// Split metrics into CRD and standard metrics
		var crdMetrics, standardMetrics []prometheus.MetricFamily
		for _, mf := range allMetricFamilies {
			if crd.IsCRDMetric(mf.Name) {
				crdMetrics = append(crdMetrics, mf)
			} else {
				standardMetrics = append(standardMetrics, mf)
			}
		}

		// Process CRD metrics if enabled
		if s.config.KSM.EnableCustomResourceMetrics && s.crdHarvester != nil && len(crdMetrics) > 0 {
			s.logger.Debugf("Exporting %d CRD metric families as dimensional metrics", len(crdMetrics))

			crdExportConfig := crd.ExportConfig{
				ClusterName: s.config.ClusterName,
				Logger:      s.logger,
				Harvester:   s.crdHarvester,
			}
			err = crd.ExportDimensionalMetrics(crdMetrics, crdExportConfig)
			if err != nil {
				s.logger.Warnf("Error exporting CRD metrics: %v", err)
			} else {
				s.logger.Debug("CRD metrics recorded to harvester successfully")
			}
		}

		// Create a cached metric families getter for the grouper using pre-fetched standard metrics
		// This avoids fetching from KSM endpoint again
		cachedGetter := func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
			// Return the pre-fetched standard metrics
			// The grouper will filter them based on its queries
			return standardMetrics, nil
		}

		// Continue with normal entity-based metric processing using cached metrics
		grouper, err := ksmGrouper.New(ksmGrouper.Config{
			MetricFamiliesGetter:       cachedGetter,
			Queries:                    metric.KSMQueries,
			ServicesLister:             s.servicesLister,
			EnableResourceQuotaSamples: s.config.KSM.EnableResourceQuotaSamples,
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

// Close will signal internal informers to stop running and flush any pending metrics.
// Safe to call multiple times.
func (s *Scraper) Close() {
	s.closeOnce.Do(func() {
		for _, ch := range s.informerClosers {
			close(ch)
		}

		// Flush any pending CRD metrics before shutdown
		if s.crdHarvester != nil {
			s.logger.Info("Flushing pending CRD metrics before shutdown...")
			// Use longer timeout for final flush to avoid losing metrics
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			s.crdHarvester.HarvestNow(ctx)
			s.logger.Info("Final CRD metrics flush completed")
		}
	})
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

func (s *Scraper) ksmURLs() ([]string, error) {
	if u := s.config.KSM.StaticURL; u != "" {
		s.logger.Debugf("Using overridden endpoint for ksm %q", u)
		return []string{u}, nil
	}

	endpoints, err := s.endpointsDiscoverer.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovering KSM endpoints: %w", err)
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
