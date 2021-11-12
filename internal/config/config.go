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
	// URL defines a static endpoint for KSM.
	StaticURL string

	// Autodiscovery settings.
	// Scheme that will be used to hit the endpoints of discovered KSM services. Defaults to http.
	Scheme string
	// If set, Port will discard all endpoints discovered that do not use this specified port. Otherwise, all endpoints will be considered.
	Port int
	// PodLabel is the selector used to filter Endpoints.
	PodLabel string
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string
	// If set, Distributed will instruct the integration to scrape all KSM endpoints rather than just the first one.
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
			StaticURL:   ksmURL,
			PodLabel:    os.Getenv("KUBE_STATE_METRIC_POD_LABEL"),
			Scheme:      schema,
			Port:        kubeStateMetricsPort,
			Namespace:   os.Getenv("KUBE_STATE_METRIC_NAMESPACE"),
			Distributed: distributedKubeStateMetrics,
		},
	}
}
