package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

// This is only a mock of the new config
type Mock struct {
	KSM
	Verbose     bool
	ClusterName string
	Interval    time.Duration
}

type KSM struct {
	// Static configuration of KSM endpoint
	Host   string
	Scheme string
	Port   int

	// Autodiscovery settings
	PodLabel    string
	Namespace   string
	Distributed bool
}

func LoadConfig() Mock {
	// strconv.ParseBool(os.Getenv("VERBOSE"))
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
		Interval:    15 * time.Second,
		KSM: KSM{
			Host:        ksmURL,
			PodLabel:    os.Getenv("KUBE_STATE_METRIC_POD_LABEL"),
			Scheme:      schema,
			Port:        kubeStateMetricsPort,
			Namespace:   os.Getenv("KUBE_STATE_METRIC_NAMESPACE"),
			Distributed: distributedKubeStateMetrics,
		},
	}
}
