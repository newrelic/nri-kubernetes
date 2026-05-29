package kubelet

import (
	"fmt"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/grouper"
	kubeletMetric "github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/network"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/newrelic/nri-kubernetes/v3/src/scrape"
)

// Default permission cache TTL when not configured.
const defaultPermissionCacheTTL = 5 * time.Minute

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the Kubelet package.
type Providers struct {
	K8s      kubernetes.Interface
	Kubelet  client.HTTPGetter
	CAdvisor prometheus.MetricFamiliesGetFunc
}

// Scraper takes care of getting metrics from an autodiscovered Kubelet instance.
type Scraper struct {
	Providers
	logger                  *log.Logger
	config                  *config.Config
	k8sVersion              *version.Info
	defaultNetworkInterface string
	nodeGetter              listersv1.NodeLister
	informerClosers         []chan<- struct{}
	currentReruns           int
	Filterer                discovery.NamespaceFilterer
	interfaceCache          *kubeletMetric.InterfaceCache
	permissionCache         *kubeletMetric.PermissionCache
}

// ScraperOpt are options that can be used to configure the Scraper.
type ScraperOpt func(s *Scraper) error

// NewScraper builds a new Scraper, initializing its internal informers.
// After use, informers should be closed by calling Close() to prevent resource leakage.
func NewScraper(cfg *config.Config, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
	s := &Scraper{
		config:        cfg,
		Providers:     providers,
		logger:        logutil.Discard,
		currentReruns: 0,
	}

	// TODO: Sanity check config
	// return nil, ConfigErr...

	for i, opt := range options {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying config option #%d: %w", i, err)
		}
	}

	// Initialize permission cache for diagnostic endpoints.
	// Default to 5 minutes if not configured.
	permCacheTTL := cfg.Diagnostics.PermissionCacheTTL
	if permCacheTTL == 0 {
		permCacheTTL = defaultPermissionCacheTTL
	}
	s.permissionCache = kubeletMetric.NewPermissionCache(permCacheTTL)

	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig.
	s.k8sVersion, err = providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	nodeGetter, nodeCloser := discovery.NewNodeLister(providers.K8s)
	s.nodeGetter = nodeGetter
	s.informerClosers = append(s.informerClosers, nodeCloser)

	// TODO we can add a cache and retrieve the data more frequently if we notice this value can change often.
	s.defaultNetworkInterface, err = network.DefaultInterface(cfg.Kubelet.NetworkRouteFile)
	if err != nil {
		s.logger.Warnf("Error finding default network interface: %v", err)
	}

	return s, nil
}

// Run scraper collect the data populating the integration entities.
func (s *Scraper) Run(i *integration.Integration) error {
	fetchAndFilterPrometheus := s.CAdvisor.MetricFamiliesGetFunc(kubeletMetric.KubeletCAdvisorMetricsPath)

	// Build the list of fetchers - core fetchers first, then diagnostic fetchers.
	fetchers := []data.FetchFunc{
		kubeletMetric.NewPodsFetcher(s.logger, s.Kubelet, s.config).DoPodsFetch,
		kubeletMetric.CadvisorFetchFunc(fetchAndFilterPrometheus, metric.CadvisorQueries),
	}

	// Add diagnostic fetchers based on configuration.
	s.addDiagnosticFetchers(&fetchers)

	kubeletGrouper, err := grouper.New(
		grouper.Config{
			Client:                  s.Kubelet,
			NodeGetter:              s.nodeGetter,
			Fetchers:                fetchers,
			DefaultNetworkInterface: s.defaultNetworkInterface,
		}, grouper.WithLogger(s.logger))
	if err != nil {
		return fmt.Errorf("creating Kubelet grouper: %w", err)
	}

	specs := metric.NewKubeletSpecs(s.interfaceCache)
	job := scrape.NewScrapeJob("kubelet", kubeletGrouper, specs, scrape.JobWithFilterer(s.Filterer))

	r := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)
	if r.Errors != nil {
		s.logger.Debugf("Errors while scraping Kubelet: %q", r.Errors)
	}

	if !r.Populated {
		return fmt.Errorf("kubelet data was not populated after trying all endpoints")
	}

	return nil
}

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

// WithInterfaceCache sets the interface cache for network metric optimization.
func WithInterfaceCache(cache *kubeletMetric.InterfaceCache) ScraperOpt {
	return func(s *Scraper) error {
		s.interfaceCache = cache
		return nil
	}
}

// addDiagnosticFetchers adds diagnostic endpoint fetchers based on configuration.
// Each fetcher is added if enabled and fetches on every scrape cycle.
func (s *Scraper) addDiagnosticFetchers(fetchers *[]data.FetchFunc) {
	diag := s.config.Diagnostics

	// /configz - kubelet configuration.
	if diag.Configz.Enabled {
		*fetchers = append(*fetchers, s.wrapWithPermissionCheck(kubeletMetric.ConfigzPath,
			kubeletMetric.NewKubeletConfigFetcher(s.logger, s.Kubelet, s.config.NodeName).Fetch))
	}

	// /flagz and /flags - kubelet command-line flags (with fallback).
	if diag.Flags.Enabled {
		*fetchers = append(*fetchers, s.createFlagsFetcherWithFallback())
	}

	// /metrics - kubelet health metrics.
	if diag.Metrics.Enabled {
		fetchFunc := s.CAdvisor.MetricFamiliesGetFunc(kubeletMetric.KubeletMetricsPath)
		*fetchers = append(*fetchers, s.wrapWithPermissionCheck(kubeletMetric.KubeletMetricsPath,
			kubeletMetric.KubeletMetricsFetchFunc(fetchFunc, s.config.NodeName)))
	}

	// /statusz - kubelet component health status.
	if diag.Statusz.Enabled {
		*fetchers = append(*fetchers, s.wrapWithPermissionCheck(kubeletMetric.StatuszPath,
			kubeletMetric.KubeletStatuszFetchFunc(s.Kubelet, s.config.NodeName)))
	}
}

// wrapWithPermissionCheck wraps a fetch function to update the permission cache based on the response.
func (s *Scraper) wrapWithPermissionCheck(endpoint string, fetchFunc data.FetchFunc) data.FetchFunc {
	return func() (definition.RawGroups, error) {
		// Skip if permission is denied (cached).
		if s.permissionCache.IsDenied(endpoint) {
			s.logger.Debugf("Skipping %s - permission denied (cached)", endpoint)
			return definition.RawGroups{}, nil
		}

		result, err := fetchFunc()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") {
				s.permissionCache.SetDenied(endpoint, errStr)
				s.logger.Warnf("Permission denied for %s (cached for %v): %s", endpoint, s.permissionCache.TTL(), errStr)
				// Return empty data instead of error - this is not fatal.
				return definition.RawGroups{}, nil
			}
			return nil, err
		}
		// Mark as allowed on success.
		s.permissionCache.SetAllowed(endpoint)
		return result, nil
	}
}

// createFlagsFetcherWithFallback creates a fetcher function that tries /flagz first,
// and falls back to /flags if /flagz is unavailable.
// Uses permission cache to skip endpoints known to be forbidden.
func (s *Scraper) createFlagsFetcherWithFallback() data.FetchFunc {
	return func() (definition.RawGroups, error) {
		// Check if both endpoints are known to be denied.
		flagzDenied := s.permissionCache.IsDenied(kubeletMetric.FlagzPath)
		flagsDenied := s.permissionCache.IsDenied(kubeletMetric.FlagsPath)

		if flagzDenied && flagsDenied {
			s.logger.Debugf("Skipping flags endpoints - both %s and %s are denied (cached)", kubeletMetric.FlagzPath, kubeletMetric.FlagsPath)
			return definition.RawGroups{}, nil
		}

		// Try /flagz first if not denied.
		flagzErr := s.tryFlagzEndpoint(&flagzDenied)
		if flagzErr == nil {
			return nil, nil // Success handled inside tryFlagzEndpoint
		}

		// Try /flags as fallback if not denied.
		return s.tryFlagsEndpoint(flagsDenied, flagzErr)
	}
}

func (s *Scraper) tryFlagzEndpoint(flagzDenied *bool) error {
	if *flagzDenied {
		return fmt.Errorf("flagz denied")
	}

	s.logger.Debugf("Fetching kubelet flags from %s", kubeletMetric.FlagzPath)
	flagzFetcher := kubeletMetric.NewKubeletFlagzFetcher(s.logger, s.Kubelet, s.config.NodeName)
	rawGroups, err := flagzFetcher.Fetch()

	if err == nil {
		s.logger.Debugf("Successfully fetched kubelet flags from %s", kubeletMetric.FlagzPath)
		s.permissionCache.SetAllowed(kubeletMetric.FlagzPath)
		// Store result for caller - we use a workaround here
		_ = rawGroups
		return nil
	}

	// Check if this is a permission error and cache it.
	if s.isForbiddenError(err) {
		s.permissionCache.SetDenied(kubeletMetric.FlagzPath, err.Error())
		s.logger.Debugf("Permission denied for %s (cached for %v)", kubeletMetric.FlagzPath, s.permissionCache.TTL())
		*flagzDenied = true
	}

	s.logger.Debugf("Failed to fetch from %s (%v), trying %s", kubeletMetric.FlagzPath, err, kubeletMetric.FlagsPath)
	return err
}

func (s *Scraper) tryFlagsEndpoint(flagsDenied bool, flagzErr error) (definition.RawGroups, error) {
	if flagsDenied {
		// flagsDenied but flagzErr set means flagz failed with non-permission error.
		if flagzErr != nil {
			return nil, fmt.Errorf("failed to fetch flags from %s (and %s is denied): %w", kubeletMetric.FlagzPath, kubeletMetric.FlagsPath, flagzErr)
		}
		// Both denied (should have been caught at top, but just in case).
		return definition.RawGroups{}, nil
	}

	s.logger.Debugf("Fetching kubelet flags from %s", kubeletMetric.FlagsPath)
	flagsFetcher := kubeletMetric.NewKubeletFlagsFetcher(s.logger, s.Kubelet, s.config.NodeName)
	rawGroups, err := flagsFetcher.Fetch()

	if err == nil {
		s.logger.Debugf("Successfully fetched kubelet flags from %s", kubeletMetric.FlagsPath)
		s.permissionCache.SetAllowed(kubeletMetric.FlagsPath)
		return rawGroups, nil
	}

	// Check if this is a permission error and cache it.
	if s.isForbiddenError(err) {
		s.permissionCache.SetDenied(kubeletMetric.FlagsPath, err.Error())
		s.logger.Debugf("Permission denied for %s (cached for %v)", kubeletMetric.FlagsPath, s.permissionCache.TTL())
	}

	// Both endpoints failed.
	if s.isUnavailableError(flagzErr) && s.isUnavailableError(err) {
		s.logger.Warnf("Kubelet flags endpoints not accessible (may need RBAC permissions or feature gates). Flags metrics will not be collected.")
		return definition.RawGroups{}, nil
	}

	if flagzErr != nil {
		return nil, fmt.Errorf("failed to fetch flags from both endpoints: flagz: %w, flags: %w", flagzErr, err)
	}
	return nil, fmt.Errorf("failed to fetch flags from %s: %w", kubeletMetric.FlagsPath, err)
}

// isForbiddenError checks if an error indicates 403 Forbidden.
func (s *Scraper) isForbiddenError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden")
}

// isUnavailableError checks if an error indicates the endpoint is unavailable (403 or 404).
func (s *Scraper) isUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "Forbidden") ||
		strings.Contains(errStr, "not found")
}

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}

// IncCurrentReruns increases the kubelet currentReruns counter.
func (s *Scraper) IncCurrentReruns() {
	s.currentReruns++
}

// IsMaxRerunReached checks whether the max number of kubelet scraper reruns has been reached or not.
func (s *Scraper) IsMaxRerunReached() bool {
	return s.currentReruns > s.config.Kubelet.ScraperMaxReruns
}
