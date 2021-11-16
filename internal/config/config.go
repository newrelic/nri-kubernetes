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
	ControlPlane
	Kubelet
	Verbose     bool
	NodeName    string
	Timeout     time.Duration
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

	Enabled bool
}

type ControlPlane struct {
	ETCD              ETCD
	APIServer         APIServer
	Scheduler         Scheduler
	ControllerManager ControllerManager
	Enabled           bool
}

type APIServer struct {
	APIServerEndpointURL string
	APIServerSecurePort  string
}

type ETCD struct {
	EtcdEndpointURL        string
	EtcdTLSSecretNamespace string
	EtcdTLSSecretName      string
}

type Scheduler struct {
	SchedulerEndpointURL string
}

type ControllerManager struct {
	ControllerManagerEndpointURL string
}

type Kubelet struct {
	Enabled bool
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

	ksmEnabled, _ := strconv.ParseBool(os.Getenv("KUBE_STATE_METRIC_ENABLED"))
	kubeleEnabled, _ := strconv.ParseBool(os.Getenv("CONTROL_PLANE_ENABLED"))
	controlPlanEnabled, _ := strconv.ParseBool(os.Getenv("KUBELET_ENABLED"))

	return Mock{
		ClusterName: os.Getenv("CLUSTER_NAME"),
		Verbose:     true,
		Timeout:     time.Millisecond * 5000,
		Interval:    15 * time.Second,
		NodeName:    os.Getenv("NRK8S_NODE_NAME"),
		Kubelet: Kubelet{
			Enabled: kubeleEnabled,
		},
		KSM: KSM{
			Enabled: ksmEnabled,

			StaticURL:   ksmURL,
			PodLabel:    os.Getenv("KUBE_STATE_METRIC_POD_LABEL"),
			Scheme:      schema,
			Port:        kubeStateMetricsPort,
			Namespace:   os.Getenv("KUBE_STATE_METRIC_NAMESPACE"),
			Distributed: distributedKubeStateMetrics,
		},
		ControlPlane: ControlPlane{
			Enabled: controlPlanEnabled,
			ETCD: ETCD{
				EtcdEndpointURL:        os.Getenv("ETCD_ENDPOINT_URL"),
				EtcdTLSSecretNamespace: os.Getenv("ETCD_ENDPOINT_SECRET_NAMESPACE"),
				EtcdTLSSecretName:      os.Getenv("ETCD_ENDPOINT_SECRET_NAME"),
			},
			APIServer: APIServer{
				APIServerEndpointURL: os.Getenv("API_SERVER_ENDPOINT_URL"),
				APIServerSecurePort:  os.Getenv("API_SERVER_SECURE_PORT"),
			},
			Scheduler: Scheduler{
				SchedulerEndpointURL: os.Getenv("SCHEDULER_ENDPOINT_URL"),
			},
			ControllerManager: ControllerManager{
				ControllerManagerEndpointURL: os.Getenv("CONTROLLER_MANAGER_ENDPOINT_URL"),
			},
		},
	}
}
