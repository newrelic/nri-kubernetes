package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/src/apiserver"
	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/controlplane"
	clientControlPlane "github.com/newrelic/nri-kubernetes/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/ksm"
	clientKsm "github.com/newrelic/nri-kubernetes/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/src/kubelet"
	clientKubelet "github.com/newrelic/nri-kubernetes/src/kubelet/client"
	metric2 "github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/src/metric"
	"github.com/newrelic/nri-kubernetes/src/scrape"
	"github.com/newrelic/nri-kubernetes/src/storage"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
	Timeout                     int    `default:"5000" help:"timeout in milliseconds for calling metrics sources"`
	ClusterName                 string `help:"Identifier of your cluster. You could use it later to filter data in your New Relic account"`
	DiscoveryCacheDir           string `default:"/var/cache/nr-kubernetes" help:"The location of the cached values for discovered endpoints. Obsolete, use CacheDir instead."`
	CacheDir                    string `default:"/var/cache/nr-kubernetes" help:"The location where to store various cached data."`
	DiscoveryCacheTTL           string `default:"1h" help:"Duration since the discovered endpoints are stored in the cache until they expire. Valid time units: 'ns', 'us', 'ms', 's', 'm', 'h'"`
	APIServerCacheTTL           string `default:"5m" help:"Duration to cache responses from the API Server. Valid time units: 'ns', 'us', 'ms', 's', 'm', 'h'. Set to 0s to disable"`
	KubeStateMetricsURL         string `help:"kube-state-metrics URL. If it is not provided, it will be discovered."`
	EtcdTLSSecretName           string `help:"Name of the secret that stores your ETCD TLS configuration"`
	EtcdTLSSecretNamespace      string `default:"default" help:"Namespace in which the ETCD TLS secret lives"`
	KubeStateMetricsPodLabel    string `help:"discover KSM using Kubernetes Labels."`
	APIServerSecurePort         string `default:"" help:"Set to query the API Server over a secure port. Disabled by default"`
	DistributedKubeStateMetrics bool   `default:"false" help:"Set to enable distributed KSM discovery. Requires that KubeStateMetricsPodLabel is set. Disabled by default."`
}

const (
	// we use '/var/cache/nr-kubernetes' as the temp cache dir rather than
	// '/var/cache/nri-kubernetes' due to the fact that this would break
	// customers setup when running unprivileged mode. Changing this value
	// would mean clients would have to update their manifest file.
	defaultCacheDir   = "/var/cache/nr-kubernetes"
	discoveryCacheDir = "discovery"
	apiserverCacheDir = "apiserver"

	defaultAPIServerCacheTTL = time.Minute * 5
	defaultDiscoveryCacheTTL = time.Hour

	integrationName    = "com.newrelic.kubernetes"
	integrationVersion = "1.13.2"
	nodeNameEnvVar     = "NRK8S_NODE_NAME"
)

var args argumentList

func getCacheDir(subDirectory string) string {
	cacheDir := args.CacheDir

	// accept the old cache directory argument if it's explicitly set
	if args.DiscoveryCacheDir != defaultCacheDir {
		cacheDir = args.DiscoveryCacheDir
	}

	return path.Join(cacheDir, subDirectory)
}

func controlPlaneJobs(
	logger *logrus.Logger,
	apiServerClient apiserver.Client,
	nodeName string,
	timeout time.Duration,
	nodeIP string,
	podsFetcher data.FetchFunc,
	k8sClient client.Kubernetes,
	etcdTLSSecretName string,
	etcdTLSSecretNamespace string,
	apiServerSecurePort string,
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

	if apiServerSecurePort != "" {
		opts = append(opts, controlplane.WithAPIServerSecurePort(apiServerSecurePort))
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

func main() {
	integration, err := sdk.NewIntegrationProtocol2(integrationName, integrationVersion, &args)
	var jobs []*scrape.Job
	exitLog := fmt.Sprintf("Integration %q exited", integrationName)
	if err != nil {
		defer log.Debug(exitLog)
		log.Fatal(err) // Global logs used as args processed inside NewIntegrationProtocol2
	}

	logger := log.New(args.Verbose)
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

	defer logger.Debug(exitLog)
	logger.Debugf("Integration %q with version %s started", integrationName, integrationVersion)
	if args.ClusterName == "" {
		logger.Panic(errors.New("cluster_name argument is mandatory"))
	}

	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		logger.Panicf("%s env var should be provided by Kubernetes and is mandatory", nodeNameEnvVar)
	}

	if !args.All && !args.Metrics {
		return
	}

	ttl, err := time.ParseDuration(args.DiscoveryCacheTTL)
	if err != nil {
		logger.WithError(err).Errorf("while parsing the cache TTL value. Defaulting to %s", defaultDiscoveryCacheTTL)
		ttl = defaultDiscoveryCacheTTL
	}

	timeout := time.Millisecond * time.Duration(args.Timeout)

	innerKubeletDiscoverer, err := clientKubelet.NewDiscoverer(nodeName, logger)
	if err != nil {
		logger.Panicf("error during Kubelet auto discovering process. %s", err)
	}
	cacheStorage := storage.NewJSONDiskStorage(getCacheDir(discoveryCacheDir))
	kubeletDiscoverer := clientKubelet.NewDiscoveryCacher(innerKubeletDiscoverer, cacheStorage, ttl, logger)

	kubeletClient, err := kubeletDiscoverer.Discover(timeout)
	if err != nil {
		logger.Panic(err)
	}
	kubeletNodeIP := kubeletClient.NodeIP()
	logger.Debugf("Kubelet node IP = %s", kubeletNodeIP)

	var ksmClients []client.HTTPClient
	var ksmNodeIP string
	if args.DistributedKubeStateMetrics {
		ksmDiscoverer, err := getMultiKSMDiscoverer(kubeletNodeIP, logger)
		if err != nil {
			logger.Panic(err)
		}
		ksmDiscoveryCache := clientKsm.NewDistributedDiscoveryCacher(ksmDiscoverer, cacheStorage, ttl, logger)
		ksmClients, err = ksmDiscoveryCache.Discover(timeout)
		logger.Debugf("found %d KSM clients:", len(ksmClients))
		for _, c := range ksmClients {
			logger.Debugf("- node IP: %s", c.NodeIP())
		}
		if err != nil {
			logger.Panic(err)
		}
		ksmNodeIP = kubeletNodeIP
	} else {
		innerKSMDiscoverer, err := getKSMDiscoverer(logger)
		if err != nil {
			logger.Panic(err)
		}
		ksmDiscoverer := clientKsm.NewDiscoveryCacher(innerKSMDiscoverer, cacheStorage, ttl, logger)
		ksmClient, err := ksmDiscoverer.Discover(timeout)
		if err != nil {
			logger.Panic(err)
		}
		ksmNodeIP = ksmClient.NodeIP()
		// we only scrape KSM when we are on the same Node as KSM
		if kubeletNodeIP == ksmNodeIP {
			ksmClients = append(ksmClients, ksmClient)
		}
	}
	logger.Debugf("KSM Node = %s", ksmNodeIP)

	ttlAPIServerCache, err := time.ParseDuration(args.APIServerCacheTTL)
	if err != nil {
		logger.WithError(err).Errorf("while parsing the api server cache TTL value. Defaulting to %s", ttlAPIServerCache)
		ttlAPIServerCache = defaultAPIServerCacheTTL
	}
	k8s, err := client.NewKubernetes(false)
	if err != nil {
		logger.Panic(err)
	}

	for _, ksmClient := range ksmClients {
		ksmGrouper := ksm.NewGrouper(ksmClient, metric.KSMQueries, logger, k8s)
		jobs = append(jobs, scrape.NewScrapeJob("kube-state-metrics", ksmGrouper, metric.KSMSpecs))
	}

	apiServerClient := apiserver.NewClient(k8s)

	if ttlAPIServerCache != time.Duration(0) {
		apiServerClient = apiserver.NewFileCacheClientWrapper(apiServerClient,
			getCacheDir(apiserverCacheDir),
			ttlAPIServerCache)
	}

	podsFetcher := metric2.NewPodsFetcher(logger, kubeletClient).FetchFuncWithCache()
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
	)

	if err != nil {
		logger.Errorf("couldn't configure control plane components jobs: %v", err)
	} else {
		jobs = append(jobs, cpJobs...)
	}

	// Kubelet is always scraped, on each node
	kubeletGrouper := kubelet.NewGrouper(kubeletClient, logger, apiServerClient,
		podsFetcher,
		metric2.CadvisorFetchFunc(kubeletClient, metric.CadvisorQueries))
	jobs = append(jobs, scrape.NewScrapeJob("kubelet", kubeletGrouper, metric.KubeletSpecs))

	successfulJobs := 0
	for _, job := range jobs {
		logger.Debugf("Running job: %s", job.Name)
		start := time.Now()
		result := job.Populate(integration, args.ClusterName, logger)
		measured := time.Now().Sub(start)
		logger.Debugf("Job %s took %s", job.Name, measured.Round(time.Millisecond))

		if result.Populated {
			successfulJobs++
		}

		if len(result.Errors) > 0 {
			logger.WithFields(logrus.Fields{"phase": "populate", "datasource": job.Name}).Debug(result.Error())
		}
	}

	if successfulJobs == 0 {
		logger.Panic("No data was populated")
	}

	if err := integration.Publish(); err != nil {
		logger.Panic(err)
	}
}

func getKSMDiscoverer(logger *logrus.Logger) (client.Discoverer, error) {

	k8sClient, err := client.NewKubernetes( /* tryLocalKubeconfig */ false)
	if err != nil {
		return nil, err
	}

	// It's important this one is before the NodeLabel selector, for backwards compatibility.
	if args.KubeStateMetricsURL != "" {
		// checking to see if KubeStateMetricsURL contains the /metrics path already.
		if strings.Contains(args.KubeStateMetricsURL, "/metrics") {
			args.KubeStateMetricsURL = strings.Trim(args.KubeStateMetricsURL, "/metrics")
		}

		logger.Debugf("Discovering KSM using static endpoint (KUBE_STATE_METRICS_URL)")
		return clientKsm.NewStaticEndpointDiscoverer(args.KubeStateMetricsURL, logger, k8sClient), nil
	}

	if args.KubeStateMetricsPodLabel != "" {
		logger.Debugf("Discovering KSM using Pod Label (KUBE_STATE_METRICS_POD_LABEL)")
		return clientKsm.NewPodLabelDiscoverer(args.KubeStateMetricsPodLabel, logger, k8sClient), nil
	}

	logger.Debugf("Discovering KSM using DNS / k8s ApiServer (default)")
	return clientKsm.NewDiscoverer(logger, k8sClient), nil
}

func getMultiKSMDiscoverer(nodeIP string, logger *logrus.Logger) (client.MultiDiscoverer, error) {
	k8sClient, err := client.NewKubernetes( /* tryLocalKubeconfig */ false)
	if err != nil {
		return nil, err
	}

	if args.KubeStateMetricsPodLabel == "" {
		return nil, errors.New("multi KSM discovery set without a KUBE_STATE_METRICS_POD_LABEL")
	}

	logger.Debugf("Discovering distributed KSMs using pod labels from KUBE_STATE_METRICS_POD_LABEL")
	return clientKsm.NewDistributedPodLabelDiscoverer(args.KubeStateMetricsPodLabel, nodeIP, logger, k8sClient), nil
}
