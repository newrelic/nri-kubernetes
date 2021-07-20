package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	clientKsm "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
)

type argumentList struct {
	args.DefaultArgumentList
	Timeout                      int    `default:"5000" help:"timeout in milliseconds for calling metrics sources"`
	ClusterName                  string `help:"Identifier of your cluster. You could use it later to filter data in your New Relic account"`
	DiscoveryCacheDir            string `default:"/var/cache/nr-kubernetes" help:"The location of the cached values for discovered endpoints. Obsolete, use CacheDir instead."`
	CacheDir                     string `default:"/var/cache/nr-kubernetes" help:"The location where to store various cached data."`
	DiscoveryCacheTTL            string `default:"1h" help:"Duration since the discovered endpoints are stored in the cache until they expire. Valid time units: 'ns', 'us', 'ms', 's', 'm', 'h'"`
	APIServerCacheTTL            string `default:"5m" help:"Duration to cache responses from the API Server. Valid time units: 'ns', 'us', 'ms', 's', 'm', 'h'. Set to 0s to disable"`
	APIServerCacheK8SVersionTTL  string `default:"3h" help:"Duration to cache the kubernetes version responses from the API Server. Valid time units: 'ns', 'us', 'ms', 's', 'm', 'h'. Set to 0s to disable"`
	EtcdTLSSecretName            string `help:"Name of the secret that stores your ETCD TLS configuration"`
	EtcdTLSSecretNamespace       string `default:"default" help:"Namespace in which the ETCD TLS secret lives"`
	DisableKubeStateMetrics      bool   `default:"false" help:"Used to disable KSM data fetching. Defaults to 'false''"`
	KubeStateMetricsURL          string `help:"kube-state-metrics URL. If it is not provided, it will be discovered."`
	KubeStateMetricsPodLabel     string `help:"discover KSM using Kubernetes Labels."`
	KubeStateMetricsPort         int    `default:"8080" help:"port to query the KSM pod. Only works together with the pod label discovery"`
	KubeStateMetricsScheme       string `default:"http" help:"scheme to query the KSM pod ('http' or 'https'). Only works together with the pod label discovery"`
	DistributedKubeStateMetrics  bool   `default:"false" help:"Set to enable distributed KSM discovery. Requires that KubeStateMetricsPodLabel is set. Disabled by default."`
	APIServerSecurePort          string `default:"" help:"Set to query the API Server over a secure port. Disabled by default"`
	SchedulerEndpointURL         string `help:"Set a custom endpoint URL for the kube-scheduler endpoint."`
	EtcdEndpointURL              string `help:"Set a custom endpoint URL for the Etcd endpoint."`
	ControllerManagerEndpointURL string `help:"Set a custom endpoint URL for the kube-controller-manager endpoint."`
	APIServerEndpointURL         string `help:"Set a custom endpoint URL for the API server endpoint."`
	NetworkRouteFile             string `help:"Route file to get the default interface from. If left empty on Linux /proc/net/route will be used by default"`
}

func (args *argumentList) cacheDir(subDirectory string) string {
	cacheDir := args.CacheDir

	// accept the old cache directory argument if it's explicitly set
	if args.DiscoveryCacheDir != defaultCacheDir {
		cacheDir = args.DiscoveryCacheDir
	}

	return path.Join(cacheDir, subDirectory)
}

func (args *argumentList) ksmDiscoverer(logger log.Logger) (client.Discoverer, error) {
	k8sClient, err := client.NewKubernetes( /* tryLocalKubeconfig */ false)
	if err != nil {
		return nil, err
	}

	// It's important this one is before the NodeLabel selector, for backwards compatibility.
	if args.KubeStateMetricsURL != "" {
		// Remove /metrics suffix if present
		args.KubeStateMetricsURL = strings.TrimSuffix(args.KubeStateMetricsURL, "/metrics")

		logger.Debugf("Discovering KSM using static endpoint (KUBE_STATE_METRICS_URL=%s)", args.KubeStateMetricsURL)

		return clientKsm.NewStaticEndpointDiscoverer(args.KubeStateMetricsURL, logger, k8sClient), nil
	}

	if args.KubeStateMetricsPodLabel != "" {
		logger.Debugf("Discovering KSM using Pod Label (KUBE_STATE_METRICS_POD_LABEL)")

		return clientKsm.NewPodLabelDiscoverer(args.KubeStateMetricsPodLabel, args.KubeStateMetricsPort, args.KubeStateMetricsScheme, logger, k8sClient), nil
	}

	logger.Debugf("Discovering KSM using DNS / k8s ApiServer (default)")

	return clientKsm.NewDiscoverer(logger, k8sClient), nil
}

func (args *argumentList) multiKSMDiscoverer(nodeIP string, logger log.Logger) (client.MultiDiscoverer, error) {
	k8sClient, err := client.NewKubernetes(false)
	if err != nil {
		return nil, err
	}

	if args.KubeStateMetricsPodLabel == "" {
		return nil, fmt.Errorf("cannot get multi KSM discovery without a KUBE_STATE_METRICS_POD_LABEL")
	}

	logger.Debugf("Discovering distributed KSMs using pod labels from KUBE_STATE_METRICS_POD_LABEL")

	return clientKsm.NewDistributedPodLabelDiscoverer(args.KubeStateMetricsPodLabel, nodeIP, logger, k8sClient), nil
}
