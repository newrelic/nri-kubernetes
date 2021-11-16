package deprecated

import (
	"os"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	clientKubelet "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	metric2 "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/network"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

var logger log.Logger

func init() {
	// TODO just stub added to keep the migrated code
	logger = log.NewStdErr(true)
}

func RunKubelet(config *config.Mock, k8s client.Kubernetes, i *integration.Integration) error {
	const (
		discoveryCacheDir        = "discovery"
		defaultDiscoveryCacheTTL = time.Hour
	)

	innerKubeletDiscoverer, err := clientKubelet.NewDiscoverer(config.NodeName, logger)
	if err != nil {
		logger.Errorf("Error during Kubelet auto discovering process: %v", err)
		os.Exit(1)
	}

	configCache := client.DiscoveryCacherConfig{
		Storage: storage.NewJSONDiskStorage(getCacheDir(discoveryCacheDir)),
		TTL:     defaultDiscoveryCacheTTL,
		Logger:  logger,
	}

	kubeletDiscoverer := clientKubelet.NewDiscoveryCacher(innerKubeletDiscoverer, configCache)

	kubeletClient, err := kubeletDiscoverer.Discover(config.Timeout)
	if err != nil {
		logger.Errorf("Error discovering kubelet: %v", err)
		os.Exit(1)
	}

	// TODO: /proc/net/route was configurable
	cacheStorage := storage.NewJSONDiskStorage(getCacheDir(discoveryCacheDir))
	defaultNetworkInterface, err := network.CachedDefaultInterface(
		logger, "/proc/net/route", cacheStorage, defaultDiscoveryCacheTTL)
	if err != nil {
		logger.Warnf("Error finding default network interface: %v", err)
	}

	kubeletGrouper := kubelet.NewGrouper(
		kubeletClient,
		logger,
		apiserver.NewClient(k8s),
		defaultNetworkInterface,
		metric2.NewPodsFetcher(logger, kubeletClient).FetchFuncWithCache(),
		metric2.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries),
	)

	K8sVersion, _ := k8s.GetClient().Discovery().ServerVersion()

	job := scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs)
	_ = job.Populate(i, config.ClusterName, logger, K8sVersion)

	return nil
}

func getCacheDir(subDirectory string) string {
	const (
		defaultCacheDir = "/var/cache/nr-kubernetes"
	)

	return path.Join(defaultCacheDir, subDirectory)
}
