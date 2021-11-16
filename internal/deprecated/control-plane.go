package deprecated

import (
	"fmt"
	"os"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	clientControlPlane "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	clientKubelet "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	metric2 "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

func RunControlPlane(config *config.Mock, k8s client.Kubernetes, i *integration.Integration) error {
	const (
		apiserverCacheDir        = "apiserver"
		discoveryCacheDir        = "discovery"
		defaultDiscoveryCacheTTL = time.Hour
		defaultAPIServerCacheTTL = time.Minute * 5
		nodeNameEnvVar           = "NRK8S_NODE_NAME"
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

	timeout := config.Timeout
	kubeletClient, err := kubeletDiscoverer.Discover(timeout)
	if err != nil {
		logger.Errorf("Error discovering kubelet: %v", err)
		os.Exit(1)
	}
	kubeletNodeIP := kubeletClient.NodeIP()
	logger.Debugf("Kubelet node IP = %s", kubeletNodeIP)

	apiServerClient := apiserver.NewClient(k8s)

	apiServerCacheTTL := defaultAPIServerCacheTTL

	if apiServerCacheTTL != time.Duration(0) {
		config := client.DiscoveryCacherConfig{
			TTL:     apiServerCacheTTL,
			Storage: storage.NewJSONDiskStorage(getCacheDir(apiserverCacheDir)),
		}
		apiServerClient = apiserver.NewFileCacheClientWrapper(apiServerClient, config)
	}

	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		logger.Errorf("%s env var should be provided by Kubernetes and is mandatory", nodeNameEnvVar)
		os.Exit(1)
	}

	cpJobs, err := controlPlaneJobs(
		logger,
		apiServerClient,
		nodeName,
		config.Timeout,
		kubeletClient.NodeIP(),
		metric2.NewPodsFetcher(logger, kubeletClient).FetchFuncWithCache(),
		k8s,
		config.ETCD.EtcdTLSSecretName,
		config.ETCD.EtcdTLSSecretNamespace,
		config.ApiServer.APIServerSecurePort,
		config.Scheduler.SchedulerEndpointURL,
		config.ETCD.EtcdEndpointURL,
		config.ControllerManager.ControllerManagerEndpointURL,
		config.ApiServer.APIServerEndpointURL,
	)

	if err != nil {
		logger.Errorf("couldn't configure control plane components jobs: %v", err)
	}

	K8sVersion, _ := k8s.GetClient().Discovery().ServerVersion()

	successfulJobs := 0
	for _, job := range cpJobs {
		logger.Debugf("Running job: %s", job.Name)
		start := time.Now()
		result := job.Populate(i, config.ClusterName, logger, K8sVersion)
		measured := time.Since(start)
		logger.Debugf("Job %s took %s", job.Name, measured.Round(time.Millisecond))

		if result.Populated {
			successfulJobs++
		}

		if len(result.Errors) > 0 {
			logger.Infof("Error populating data from %s: %v", job.Name, result.Error())
		}
	}

	if successfulJobs == 0 {
		logger.Errorf("No data was populated")
		os.Exit(1)
	}
	return nil
}

func controlPlaneJobs(
	logger log.Logger,
	apiServerClient apiserver.Client,
	nodeName string,
	timeout time.Duration,
	nodeIP string,
	podsFetcher data.FetchFunc,
	k8sClient client.Kubernetes,
	etcdTLSSecretName string,
	etcdTLSSecretNamespace string,
	apiServerSecurePort string,
	schedulerEndpointURL string,
	etcdEndpointURL string,
	controllerManagerEndpointURL string,
	apiServerEndpointURL string,
) ([]*scrape.Job, error) {
	nodeInfo, err := apiServerClient.GetNodeInfo(nodeName)
	if err != nil {
		return nil, fmt.Errorf("couldn't query ApiServer server: %v", err)
	}

	if !nodeInfo.IsMasterNode() {
		return nil, nil
	}

	var opts []controlplane.ComponentOption
	if etcdTLSSecretName != "" {
		opts = append(opts, controlplane.WithEtcdTLSConfig(etcdTLSSecretName, etcdTLSSecretNamespace))
	}

	// Make sure API Server Secure port is used first for backwards compatibility.
	if apiServerSecurePort != "" && apiServerEndpointURL != "" {
		return nil, fmt.Errorf("api server secure port and api server endpoint URL can not both be set")
	} else if apiServerSecurePort != "" {
		opts = append(opts, controlplane.WithAPIServerSecurePort(apiServerSecurePort))
	} else if apiServerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.APIServer, apiServerEndpointURL))
	}

	if schedulerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.Scheduler, schedulerEndpointURL))
	}

	if etcdEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.Etcd, etcdEndpointURL))
	}

	if controllerManagerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.ControllerManager, controllerManagerEndpointURL))
	}

	var jobs []*scrape.Job
	for _, component := range controlplane.BuildComponentList(opts...) {

		// Components will be skipped if their configuration is not correct.
		if component.Skip {
			logger.Debugf("Skipping job creation for component %s: %s", component.Name, component.SkipReason)
			continue
		}

		componentDiscoverer := clientControlPlane.NewComponentDiscoverer(component, logger, nodeIP, podsFetcher, k8sClient)
		componentClient, err := componentDiscoverer.Discover(timeout)
		if err != nil {
			logger.Errorf("control plane component %s discovery failed: %v", component.Name, err)
		}

		c := componentClient.(*clientControlPlane.ControlPlaneComponentClient)

		if !c.IsComponentRunningOnNode {
			logger.Debugf(
				"Could not find component %s on this master node, skipping job. ",
				component.Name,
			)
			continue
		}

		componentGrouper := controlplane.NewComponentGrouper(
			componentClient,
			component.Queries,
			logger,
			c.PodName,
		)
		jobs = append(
			jobs,
			scrape.NewScrapeJob(string(component.Name), componentGrouper, component.Specs),
		)
	}

	return jobs, nil
}
