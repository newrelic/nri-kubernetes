package main

import (
	"fmt"
	"os"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v2/src/apiserver"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	controlplaneclient "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/featureflag"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmclient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet"
	kubeletclient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	kubeletmetric "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/network"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

const (
	// we use '/var/cache/nr-kubernetes' as the temp cache dir rather than
	// '/var/cache/nri-kubernetes' due to the fact that this would break
	// customers setup when running unprivileged mode. Changing this value
	// would mean clients would have to update their manifest file.
	defaultCacheDir             = "/var/cache/nr-kubernetes"
	discoveryCacheDir           = "discovery"
	apiserverCacheDir           = "apiserver"
	apiserverCacheDirK8sVersion = "apiserverK8SVersion"

	defaultAPIServerCacheTTL           = time.Minute * 5
	defaultAPIServerCacheK8SVersionTTL = time.Hour * 3
	defaultDiscoveryCacheTTL           = time.Hour

	integrationName = "com.newrelic.kubernetes"
	nodeNameEnvVar  = "NRK8S_NODE_NAME"
)

var (
	integrationVersion    = "dev"
	integrationCommitHash = "unknown"
)

func run() error {
	args := &argumentList{}

	intgr, err := integration.New(integrationName, integrationVersion, integration.Args(args))
	var jobs []*scrape.Job
	if err != nil {
		return fmt.Errorf("creating integration: %w", err)
	}

	logger := log.NewStdErr(args.Verbose)

	defer func() {
		if r := recover(); r != nil {
			recErr, ok := r.(*logrus.Entry)
			if ok {
				recErr.Fatal(recErr.Message)
			} else {
				panic(r)
			}
		}
	}()

	logger.Debugf("Integration %q ver. %s (git %s) started", integrationName, integrationVersion, integrationCommitHash)
	if args.ClusterName == "" {
		return fmt.Errorf("cluster_name argument is mandatory")
	}

	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("could not find %q in env, which should have been set by Kubernetes", nodeNameEnvVar)
	}

	if !args.HasMetrics() {
		return fmt.Errorf("integration only supports the -metrics or -all commands")
	}

	ttl, err := time.ParseDuration(args.DiscoveryCacheTTL)
	if err != nil {
		logger.Errorf("Error while parsing the cache TTL value, defaulting to %s: %v", defaultDiscoveryCacheTTL, err)
		ttl = defaultDiscoveryCacheTTL
	}

	timeout := time.Millisecond * time.Duration(args.Timeout)

	innerKubeletDiscoverer, err := kubeletclient.NewDiscoverer(nodeName, logger)
	if err != nil {
		return fmt.Errorf("Error during Kubelet auto discovering process: %v", err)
	}

	cacheStorage := storage.NewJSONDiskStorage(args.cacheDir(discoveryCacheDir))

	defaultNetworkInterface, err := network.CachedDefaultInterface(logger, args.NetworkRouteFile, cacheStorage, ttl)
	if err != nil {
		logger.Warnf("Error finding default network interface: %v", err)
	}

	kubeletDiscoverer := kubeletclient.NewDiscoveryCacher(innerKubeletDiscoverer, cacheStorage, ttl, logger)

	kubeletClient, err := kubeletDiscoverer.Discover(timeout)
	if err != nil {
		return fmt.Errorf("discovering kubelet: %w", err)
	}

	kubeletNodeIP := kubeletClient.NodeIP()
	logger.Debugf("Kubelet node IP = %s", kubeletNodeIP)

	k8s, err := client.NewKubernetes(false)
	if err != nil {
		return fmt.Errorf("building kubernetes client: %w", err)
	}

	if !args.DisableKubeStateMetrics {
		var ksmClients []client.HTTPClient
		var ksmNodeIP string
		if args.DistributedKubeStateMetrics {
			ksmDiscoverer, err := args.multiKSMDiscoverer(kubeletNodeIP, logger)
			if err != nil {
				return fmt.Errorf("getting multiKSM discoverer: %v", err)
			}

			ksmDiscoveryCache := ksmclient.NewDistributedDiscoveryCacher(ksmDiscoverer, cacheStorage, ttl, logger)
			ksmClients, err = ksmDiscoveryCache.Discover(timeout)
			if err != nil {
				return fmt.Errorf("discovering distributed KSM: %w", err)
			}

			logger.Debugf("found %d KSM clients:", len(ksmClients))
			for _, c := range ksmClients {
				logger.Debugf("- node IP: %s", c.NodeIP())
			}
			ksmNodeIP = kubeletNodeIP
		} else {
			innerKSMDiscoverer, err := args.ksmDiscoverer(logger)
			if err != nil {
				return fmt.Errorf("getting standalone KSM discoverer: %v", err)
			}

			ksmDiscoverer := ksmclient.NewDiscoveryCacher(innerKSMDiscoverer, cacheStorage, ttl, logger)
			ksmClient, err := ksmDiscoverer.Discover(timeout)
			if err != nil {
				return fmt.Errorf("discovering standalone KSM: %v", err)
			}

			ksmNodeIP = ksmClient.NodeIP()
			// we only scrape KSM when we are on the same Node as KSM
			if kubeletNodeIP == ksmNodeIP {
				ksmClients = append(ksmClients, ksmClient)
			}
		}
		logger.Debugf("Found KSM in node %q", ksmNodeIP)

		for _, ksmClient := range ksmClients {
			ksmGrouper := ksm.NewGrouper(ksmClient, metric.KSMQueries, logger, k8s)
			jobs = append(jobs, scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs))
		}
	}

	apiServerClient := apiserver.NewClient(k8s)

	ttlAPIServerCacheK8SVersion, err := time.ParseDuration(args.APIServerCacheK8SVersionTTL)
	if err != nil {
		logger.Errorf(
			"Error while parsing the api server cache TTL value for the kubernetes server version, defaulting to %s: %v",
			defaultAPIServerCacheK8SVersionTTL, err,
		)
		ttlAPIServerCacheK8SVersion = defaultAPIServerCacheK8SVersionTTL
	}

	var apiServerClientK8sVersion apiserver.Client
	if ttlAPIServerCacheK8SVersion != time.Duration(0) {
		apiServerClientK8sVersion = apiserver.NewFileCacheClientWrapper(
			apiServerClient,
			args.cacheDir(apiserverCacheDirK8sVersion),
			ttlAPIServerCacheK8SVersion,
		)
	} else {
		apiServerClientK8sVersion = apiServerClient
	}

	k8sVersion, err := apiServerClientK8sVersion.GetServerVersion()
	if err != nil {
		logger.Errorf("Error getting the kubernetes server version: %v", err)
	}

	enableStaticPodsStatus := featureflag.StaticPodsStatus(k8sVersion)

	ttlAPIServerCache, err := time.ParseDuration(args.APIServerCacheTTL)
	if err != nil {
		logger.Errorf("Error while parsing the api server cache TTL value, defaulting to %s: %v", defaultAPIServerCacheTTL, err)
		ttlAPIServerCache = defaultAPIServerCacheTTL
	}

	if ttlAPIServerCache != time.Duration(0) {
		apiServerClient = apiserver.NewFileCacheClientWrapper(apiServerClient,
			args.cacheDir(apiserverCacheDir),
			ttlAPIServerCache)
	}

	podsFetcher := kubeletmetric.NewPodsFetcher(logger, kubeletClient, enableStaticPodsStatus).FetchFuncWithCache()
	cpJobs, err := controlPlaneJobs(
		logger,
		apiServerClient,
		nodeName,
		timeout,
		kubeletNodeIP,
		podsFetcher,
		k8s,
		args.EtcdTLSSecretName,
		args.EtcdTLSSecretNamespace,
		args.APIServerSecurePort,
		args.SchedulerEndpointURL,
		args.EtcdEndpointURL,
		args.ControllerManagerEndpointURL,
		args.APIServerEndpointURL,
	)

	if err != nil {
		logger.Errorf("Couldn't configure control plane components jobs, skipping control plane monitoring: %v", err)
	} else {
		jobs = append(jobs, cpJobs...)
	}

	// Kubelet is always scraped, on each node
	kubeletGrouper := kubelet.NewGrouper(
		kubeletClient,
		logger,
		apiServerClient,
		defaultNetworkInterface,
		podsFetcher,
		kubeletmetric.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries),
	)
	jobs = append(jobs, scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs))

	successfulJobs := 0
	for _, job := range jobs {
		logger.Debugf("Running job: %s", job.Name)
		start := time.Now()
		result := job.Populate(intgr, args.ClusterName, logger, k8sVersion)
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
		return fmt.Errorf("getting k8s data, all jobs failed")
	}

	if err := intgr.Publish(); err != nil {
		return fmt.Errorf("rendering integration output: %w", err)
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

		componentDiscoverer := controlplaneclient.NewComponentDiscoverer(component, logger, nodeIP, podsFetcher, k8sClient)
		componentClient, err := componentDiscoverer.Discover(timeout)
		if err != nil {
			logger.Errorf("control plane component %s discovery failed: %v", component.Name, err)

			continue
		}

		c := componentClient.(*controlplaneclient.ControlPlaneComponentClient)

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
