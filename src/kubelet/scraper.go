package kubelet

import (
	"fmt"

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
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/grouper"
	kubeletMetric "github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/network"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/newrelic/nri-kubernetes/v3/src/scrape"
)

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
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
// Close() to prevent resource leakage.
func NewScraper(config *config.Config, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
	s := &Scraper{
		config:        config,
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

	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig
	s.k8sVersion, err = providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	nodeGetter, nodeCloser := discovery.NewNodeLister(providers.K8s)
	s.nodeGetter = nodeGetter
	s.informerClosers = append(s.informerClosers, nodeCloser)

	//TODO we can add a cache and retrieve the data more frequently if we notice this value can change often
	s.defaultNetworkInterface, err = network.DefaultInterface(config.Kubelet.NetworkRouteFile)
	if err != nil {
		s.logger.Warnf("Error finding default network interface: %v", err)
	}

	return s, nil
}

// Run scraper collect the data populating the integration entities
func (s *Scraper) Run(i *integration.Integration) error {
	fetchAndFilterPrometheus := s.CAdvisor.MetricFamiliesGetFunc(kubeletMetric.KubeletCAdvisorMetricsPath)

	kubeletGrouper, err := grouper.New(
		grouper.Config{
			Client:     s.Kubelet,
			NodeGetter: s.nodeGetter,
			Fetchers: []data.FetchFunc{
				kubeletMetric.NewPodsFetcher(s.logger, s.Kubelet, s.config).DoPodsFetch,
				kubeletMetric.CadvisorFetchFunc(fetchAndFilterPrometheus, metric.CadvisorQueries),
			},
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

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}

// Increase the kubelet currentReruns counter.
func (s *Scraper) IncCurrentReruns() {
	s.currentReruns++
}

// Check whether the max number of kubulet scraper reruns has been reached or not.
func (s *Scraper) IsMaxRerunReached() bool {
	return s.currentReruns > s.config.Kubelet.ScraperMaxReruns
}
