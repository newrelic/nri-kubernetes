package kubelet

import (
	"fmt"
	"io"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/grouper"
	metric2 "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/network"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
)

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the Kubelet package.
type Providers struct {
	K8s      kubernetes.Interface
	Kubelet  kubeletClient.HTTPGetter
	CAdvisor kubeletClient.MetricFamiliesGetter
}

// Scraper takes care of getting metrics from an autodiscovered Kubelet instance.
type Scraper struct {
	Providers
	logger                  log.Logger
	config                  *config.Mock
	k8sVersion              *version.Info
	defaultNetworkInterface string
	nodeGetter              discovery.NodeGetter
	informerClosers         []chan<- struct{}
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
// Close() to prevent resource leakage.
func NewScraper(config *config.Mock, providers Providers, options ...ScraperOpt) (*Scraper, error) {
	var err error
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

	// TODO If this could change without a restart of the pod we should run it each time we scrape data,
	// possibly with a reasonable cache Es: NewCachedDiscoveryClientForConfig
	s.k8sVersion, err = providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}

	nodeGetter, nodeCloser := discovery.NewNodesGetter(providers.K8s)
	s.nodeGetter = nodeGetter
	s.informerClosers = append(s.informerClosers, nodeCloser)

	s.defaultNetworkInterface, err = network.DefaultInterface(config.NetworkRouteFile)
	if err != nil {
		s.logger.Warnf("Error finding default network interface: %v", err)
	}

	return s, nil
}

func (s *Scraper) Run(i *integration.Integration) error {
	kubeletGrouper, err := grouper.New(
		grouper.Config{
			Client:     s.Kubelet,
			NodeGetter: s.nodeGetter,
			Fetchers: []data.FetchFunc{
				metric2.NewPodsFetcher(s.logger, s.Kubelet).DoPodsFetch,
				metric2.CadvisorFetchFunc(s.CAdvisor, metric.CadvisorQueries),
			},
			DefaultNetworkInterface: s.defaultNetworkInterface,
		}, grouper.WithLogger(s.logger))
	if err != nil {
		return fmt.Errorf("creating Kubelet grouper: %w", err)
	}

	job := scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs)

	r := job.Populate(i, s.config.ClusterName, s.logger, s.k8sVersion)
	if r.Errors != nil {
		s.logger.Debugf("Errors while scraping Kubelet: %q", r.Errors)
	}

	if !r.Populated {
		return fmt.Errorf("kubelet data was not populated after trying all endpoints")
	}

	return nil
}

func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

// Close will signal internal informers to stop running.
func (s *Scraper) Close() {
	for _, ch := range s.informerClosers {
		close(ch)
	}
}
