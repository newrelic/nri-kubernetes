package kubelet

import (
	"fmt"
	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	metric2 "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/network"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"io"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
)

// Providers is a struct holding pointers to all the clients Scraper needs to get data from.
// TODO: Extract this out of the Kubelet package.
type Providers struct {
	K8s     kubernetes.Interface
	Kubelet kubeletClient.HTTPClient
}

// Scraper takes care of getting metrics from an autodiscovered Kubelet instance.
type Scraper struct {
	Providers
	logger                  log.Logger
	config                  *config.Mock
	k8sVersion              *version.Info
	clusterName             string
	defaultNetworkInterface string
}

// ScraperOpt are options that can be used to configure the Scraper
type ScraperOpt func(s *Scraper) error

// NewScraper builds a new Scraper, initializing its internal informers. After use, informers should be closed by calling
// Close() to prevent resource leakage.
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

	k8sVersion, err := providers.K8s.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching K8s version: %w", err)
	}
	s.k8sVersion = k8sVersion
	s.clusterName = config.ClusterName

	// TODO: /proc/net/route was configurable
	defaultNetworkInterface, err := network.DefaultInterface("/proc/net/route")
	if err != nil {
		s.logger.Warnf("Error finding default network interface: %v", err)
	}
	s.defaultNetworkInterface = defaultNetworkInterface

	return s, nil
}

func (s *Scraper) Run(i *integration.Integration) error {
	kubeletGrouper := NewGrouper(
		s.Kubelet,
		s.logger,
		apiserver.NewClient(s.K8s),
		s.defaultNetworkInterface,
		metric2.NewPodsFetcher(s.logger, s.Kubelet).FetchFuncWithCache(),
		metric2.CadvisorFetchFunc(s.Kubelet, metric.CadvisorQueries),
	)

	job := scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs)
	_ = job.Populate(i, s.clusterName, s.logger, s.k8sVersion)
	//todo manage proprerly

	return nil
}

func WithLogger(logger log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}
