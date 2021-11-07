package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
)

//This is only a mock of the new config
type Mock struct {
	KSMConfig   KSMConfig
	Verbose     bool
	ClusterName string
}

type KSMConfig struct {
	KubeStateMetricsURL         string
	KubeStateMetricsPodLabel    string
	KubeStateMetricsScheme      string
	KubeStateMetricsPort        int
	KubeStateMetricsNamespace   string
	DistributedKubeStateMetrics bool
}

func LoadConfig() Mock {
	//strconv.ParseBool(os.Getenv("VERBOSE"))
	kubeStateMetricsPort, _ := strconv.Atoi(os.Getenv("KUBE_STATE_METRIC_PORT"))
	distributedKubeStateMetrics, _ := strconv.ParseBool(os.Getenv("DISTRIBUTED_KUBE_STATE_METRIC"))
	schema := "http"

	if os.Getenv("KUBE_STATE_METRIC_SCHEME") != "" {
		schema = os.Getenv("KUBE_STATE_METRIC_SCHEME")
	}

	var ksmURL string
	if u, err := url.Parse(os.Getenv("KUBE_STATE_METRIC_URL")); err != nil {
		ksmURL = net.JoinHostPort(u.Host, u.Port())
		schema = u.Scheme
	}

	return Mock{
		ClusterName: os.Getenv("CLUSTER_NAME"),
		Verbose:     true,
		KSMConfig: KSMConfig{
			KubeStateMetricsURL:         ksmURL,
			KubeStateMetricsPodLabel:    os.Getenv("KUBE_STATE_METRIC_POD_LABEL"),
			KubeStateMetricsScheme:      schema,
			KubeStateMetricsPort:        kubeStateMetricsPort,
			KubeStateMetricsNamespace:   os.Getenv("KUBE_STATE_METRIC_NAMESPACE"),
			DistributedKubeStateMetrics: distributedKubeStateMetrics,
		},
	}
}
