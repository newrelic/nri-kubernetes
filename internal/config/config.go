package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultConfigFileName   = "nri-kubernetes"
	DefaultConfigFolderName = "/etc/newrelic-infra"

	DefaultTimeout          = 10 * time.Second
	DefaultRetries          = 3
	DefaultScraperMaxReruns = 4
	DefaultAgentTimeout     = 3 * time.Second
	DefaultProbeTimeout     = 90 * time.Second
	DefaultProbeBackoff     = 5 * time.Second

	DefaultNetworkRouteFile = "/host/proc/1/net/route"

	SinkTypeHTTP   = "http"
	SinkTypeStdout = "stdout"
)

type Config struct {
	// Verbose is a shortcut flag to LogLevel=Debug
	Verbose bool `mapstructure:"verbose"`
	// LogLevel defines the logrus.Logger log level used by the integration.
	LogLevel string `mapstructure:"logLevel"`
	// ClusterName is a unique, human-readable name for the cluster. Will be used to qualify entities and displayNames.
	ClusterName string `mapstructure:"clusterName"`
	// KubeconfigPath is the path to a local kube/config file. If empty, in-cluster config will be used.
	KubeconfigPath string `mapstructure:"kubeconfigPath"`
	// NodeIP is the main IP for the node where the integration is running. Used to connect to the Kubelet.
	NodeIP string `mapstructure:"nodeIP"`
	// TestConnectionEndpoint is the endpoint to use to test http connectivity. useful for environments where healthz is inaccessible like GKE-autopilot.
	TestConnectionEndpoint string `mapstructure:"testConnectionEndpoint"`
	// NodeName is the name of the node where the integration is running. Used to retrieve node info from the API
	// Server, and to connect to the Kubelet through the API Server proxy if direct connection fails.
	NodeName string `mapstructure:"nodeName"`
	// Interval is the time the integration will wait between metric collection runs.
	Interval time.Duration `mapstructure:"interval"`

	// Sink defines where the integration will report the metrics to.
	Sink struct {
		// Type allows selecting which of the supported sinks will be used by the integration.
		// Supported values are `http` and `stdout`.
		Type string `mapstructure:"type"`
		// HTTP stores the configuration for the HTTP sink.
		HTTP HTTPSink `mapstructure:"http"`
	} `mapstructure:"sink"`

	// ControlPlane defines config options for the control plane scraper.
	ControlPlane `mapstructure:"controlPlane"`
	// Kubelet defines config options for the kubelet scraper.
	Kubelet `mapstructure:"kubelet"`
	// KSM defines config options for the kube-state-metrics scraper.
	KSM `mapstructure:"ksm"`

	// NamespaceSelector defines custom monitoring filtering for namespaces.
	NamespaceSelector *NamespaceSelector `mapstructure:"namespaceSelector"`
}

// HTTPSink stores the configuration for the HTTP sink.
type HTTPSink struct {
	// Port to be used for the HTTP sink.
	Port int `mapstructure:"port"`
	// Timeout is the amount of time to wait before giving up the connection to the HTTP sink.
	Timeout time.Duration `mapstructure:"timeout"`
	// Retries is the maximum number of attempts to connect to the HTTP sink if the connection fails before giving up
	// and exiting.
	Retries int `mapstructure:"retries"`
	// TLS allows to configure TLS encryption and authentication for the HTTP sink.
	TLS TLSConfig `mapstructure:"tls"`
	// ProbeTimeout is the amount of time the main func to probe infra agent sidecar before giving up and restarting during scraper agent starting.
	ProbeTimeout time.Duration `mapstructure:"probeTimeout"`
	// ProbeBackoff is the amount of time the main func to backoff when it fails to probe infra agent sidecar.
	ProbeBackoff time.Duration `mapstructure:"probeBackoff"`
}

type TLSConfig struct {
	// Enabled dictates whether TLS is used to connect to the HTTP sink.
	Enabled bool `mapstructure:"enabled"`
	// CertPath is the path to a PEM-encoded TLS certificate that will be presented to the server.
	CertPath string `mapstructure:"certPath"`
	// KeyPath is the path to the PEM-encoded private key for CertPath.
	KeyPath string `mapstructure:"keyPath"`
	// CAPath is the path to the PEM-encoded CA certificate that will be used to validate the server certificate.
	CAPath string `mapstructure:"caPath"`
}

// KSM contains configuration options for the KSM scraper.
type KSM struct {
	// Enabled controls whether KSM scraping will be attempted.
	Enabled bool `mapstructure:"enabled"`
	// StaticURL overrides KSM autodiscovery and forces the integration to just connect to this URL instead.
	StaticURL string `mapstructure:"staticURL"`
	// Scheme is the scheme that will be used for autodiscovered KSM service endpoints.
	// If empty, ksm.defaultScheme (`http`) will be assumed.
	Scheme string `mapstructure:"scheme"`
	// Port allows to filter autodiscovered endpoints. If non-zero, only endpoints using Port will be considered.
	Port int `mapstructure:"port"`
	// Selector is a string-encoded label selector to narrow KSM service discovery.
	// If empty, ksm.defaultLabelSelector is used.
	Selector string `mapstructure:"selector"`
	// Namespace allows limiting KSM autodiscovery to a particular namespace.
	// If empty, the integration will look for KSM service endpoints matching the Selector above on all namespaces.
	Namespace string `mapstructure:"namespace"`
	// Distributed is an EXPERIMENTAL flag that will cause the integration to collect metrics from all autodiscovered
	// KSM endpoints, instead of just the first one.
	Distributed bool `mapstructure:"distributed"`
	// Timeout controls the timeout for the requests to the KSM service.
	Timeout time.Duration `mapstructure:"timeout"`
	// Retries controls how many times the integration will attempt to connect to the KSM endpoint before giving up.
	Retries int `mapstructure:"retries"`
	// Enable collection of ResourceQuota metrics as samples.
	EnableResourceQuotaSamples bool `mapstructure:"enableResourceQuotaSamples"`
	// Discovery allows to configure timing aspects of KSM discovery.
	Discovery struct {
		// BackoffDelay controls how much time to wait between attempts to find the KSM service in the cluster.
		BackoffDelay time.Duration `mapstructure:"backoffDelay"`
		// Timeout controls how much time the integration will wait for a KSM service to appear before giving up.
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"discovery"`
}

// Kubelet contains config options for the Kubelet scraper.
type Kubelet struct {
	// Enabled controls whether Kubelet scraping will be attempted.
	Enabled bool `mapstructure:"enabled"`
	// FetchPodsFromKubeService fetches pods from the kube service instead of local node.
	FetchPodsFromKubeService bool `mapstructure:"fetchPodsFromKubeService"`
	// Port controls which port will be used to connect to the kubelet.
	// If zero, the kubelet port will be discovered from the status of the Node object in the API Server.
	Port int32 `mapstructure:"port"`
	// Scheme controls the scheme to be used to connect to the kubelet.
	// If empty, the integration will try to guess the scheme based on the port number, by checking if this number is
	// either the well-known http or https port for the kubelet.
	// If Scheme is not specified and the Port is non-standard, the integration will fail to connect.
	Scheme string `mapstructure:"scheme"`
	// Path to the file containing the network routes of the system, used to figure out the default network interface
	// for which metrics will be collected.
	// Defaults to /proc/net/route.
	NetworkRouteFile string `mapstructure:"networkRouteFile"`
	// Timeout controls the timeout for the requests to the kubelet.
	Timeout time.Duration `mapstructure:"timeout"`
	// Retries controls how many times the integration will attempt to connect to the kubelet before giving up.
	Retries int `mapstructure:"retries"`
	// ScraperMaxReruns controls how many times the integration will attempt to
	// run kubelet scraper when runtime error happens before giving up.
	ScraperMaxReruns int `mapstructure:"scraperMaxReruns"`
}

// ControlPlane contains config options for the control plane scraper.
type ControlPlane struct {
	// Enabled controls whether control plane scraping will be attempted, for any component.
	Enabled bool `mapstructure:"enabled"`
	// ETCD contains configuration for the etcd scraper.
	ETCD ControlPlaneComponent `mapstructure:"etcd"`
	// APIServer contains configuration for the API server scraper.
	APIServer ControlPlaneComponent `mapstructure:"apiServer"`
	// ControllerManager contains configuration for the controller manager scraper.
	ControllerManager ControlPlaneComponent `mapstructure:"controllerManager"`
	// Scheduler contains configuration for the scheduler scraper.
	Scheduler ControlPlaneComponent `mapstructure:"scheduler"`
	// Timeout controls the timeout for the requests to control plane endpoints.
	Timeout time.Duration `mapstructure:"timeout"`
	// Retries controls how many times the integration will attempt to connect to control plane components before giving up.
	Retries int `mapstructure:"retries"`
}

// ControlPlaneComponent contains the config for a control plane component.
type ControlPlaneComponent struct {
	// Enabled controls whether this particular component should be scraped.
	Enabled bool `mapstructure:"enabled"`
	// StaticEndpoint contains an Endpoint configuration. If set, Autodiscover will not be attempted and the integration
	// will contact this endpoint directly instead.
	// Please note that failure to connect to a StaticEndpoint is considered a fatal error and will cause the
	// integration to exit with a non-zero code.
	StaticEndpoint *Endpoint `mapstructure:"staticEndpoint"`
	// Autodiscover contains one or more criteria for discovering control plane endpoints. Entries will be iterated in
	// order, with the following rules:
	// 1. If an entry's criteria (Selector, Namespace, MatchNode) does not match any pod, the next entry will be tried.
	// 2. If none of the entries matches any pod, the integration will not error but keep probing in case matching pods appear.
	// 3. If an entry's criteria more than one pod, only the first match will be considered.
	// 4. Endpoints are tried in order for a matching pod, until metrics can be read successfully from one of them.
	// 5. If all endpoints for a matching fail, no more entries will be processed, and the integration will keep probing in case matching pods appear..
	Autodiscover []AutodiscoverControlPlane `mapstructure:"autodiscover"`
}

// AutodiscoverControlPlane stores criteria for matching a control plane pod.
type AutodiscoverControlPlane struct {
	// Namespace restrict matching pods to a certain namespace.
	// If empty, all namespaces will be considered.
	Namespace string `mapstructure:"namespace"`
	// Selector is a string-encoded label selector to match pods for a particular component.
	Selector string `mapstructure:"selector"`
	// MatchNode is a flag that when set, will discard pods discovered that are not running in the same node as the
	// integration. This flag is useful when running the control plane scraper as a DaemonSet with `hostNetwork`, where
	// the components will be contacted through `localhost`.
	MatchNode bool `mapstructure:"matchNode"`
	// Endpoints is a list of endpoints to try if a pod matching the above criteria is found.
	Endpoints []Endpoint `mapstructure:"endpoints"`
}

// Endpoint contains information about how to perform a request to a component.
type Endpoint struct {
	// URL is the full URL (with scheme and port) to attempt the connection to.
	URL string `mapstructure:"url"`
	// Auth specifies if authentication will be attempted against this endpoint.
	Auth *Auth `mapstructure:"auth"`
	// InsecureSkipVerify allows to skip verification of TLS certificates.
	// If URL scheme is not https, this field is ignored.
	InsecureSkipVerify bool `mapstructure:"insecureSkipVerify"`
}

// Auth specifies if authentication will be attempted against this endpoint.
type Auth struct {
	// Type specifies which authentication mechanism will be used. Supported values are `mtls` and `token`.
	// If `token` is specified, connection will be performed using the ServiceAccount bearer token mounted in the pod.
	// If `mtls` is specified, tls certificates will be pulled from secrets as sefined in the MTLS struct.
	Type string `mapstructure:"type"`
	// MTLS contains instructions on where to fetch TLS certificates from when connecting to control plane endpoints.
	// These secrets are fetched using the Kubernetes API and the pod must have a ServiceAccount token holding the
	// appropriate RBAC roles to perform this operation.
	MTLS *MTLS `mapstructure:"mtls"`
}

type MTLS struct {
	// TLSSecretName is the name of the secret containing TLS certificate, private key, and CA certificate that will be
	// used to perform mutual TLS authentication with the endpoint.
	// It is recommended for this secret to be of type: kubernetes.io/tls.
	TLSSecretName string `mapstructure:"secretName"`
	// TLSSecretNamespace is the namespace where the secret above is located.
	TLSSecretNamespace string `mapstructure:"secretNamespace"`
}

// NamespaceSelector contains config options for filtering namespaces.
type NamespaceSelector struct {
	// MatchLabels is a list of labels to filter namespaces with.
	MatchLabels map[string]interface{} `mapstructure:"matchLabels"`
	// MatchExpressions is a list of namespaces selector requirements.
	MatchExpressions []Expression `mapstructure:"matchExpressions"`
}

// Expression hold the values to generate an expression to filter namespaces by MatchExpressions.
type Expression struct {
	// Key it the key of the label.
	Key string `mapstructure:"key"`
	// Operator holds either an inclusion (NotIn) or exclusion (In) value.
	Operator string `mapstructure:"operator"`
	// Values is a slice of values related to the key.
	Values []interface{} `mapstructure:"values"`
}

func (e *Expression) String() (string, error) {
	var values []string

	for _, val := range e.Values {
		str, ok := val.(string)
		// This should never happen as each expression value type is checked when loading the config.
		if !ok {
			return "", fmt.Errorf("parsing expression invalid value: %v, type: %T", val, val)
		}

		values = append(values, str)
	}

	return fmt.Sprintf("%s %s (%s)", e.Key, strings.ToLower(e.Operator), strings.Join(values, ",")), nil
}

func LoadConfig(filePath string, fileName string) (*Config, error) {
	// Update default delimiter as with the new namespaceSelector config, some labels may come in the form of
	// newrelic.com/scrape, so the key was split in a sub-map on a "." basis.
	v := viper.NewWithOptions(viper.KeyDelimiter("|"))

	// We need to assure that defaults have been set in order to bind env variables.
	// https://github.com/spf13/viper/issues/584
	v.SetDefault("clusterName", "cluster")
	v.SetDefault("verbose", false)
	v.SetDefault("kubelet|networkRouteFile", DefaultNetworkRouteFile)
	v.SetDefault("nodeName", "node")
	v.SetDefault("nodeIP", "node")
	v.SetDefault("testConnectionEndpoint", "/healthz")

	// Sane connection defaults
	v.SetDefault("sink|type", SinkTypeHTTP)
	v.SetDefault("sink|http|port", 0)
	v.SetDefault("sink|http|timeout", DefaultAgentTimeout)
	v.SetDefault("sink|http|retries", DefaultRetries)
	v.SetDefault("sink|http|probeTimeout", DefaultProbeTimeout)
	v.SetDefault("sink|http|probeBackoff", DefaultProbeBackoff)

	v.SetDefault("kubelet|timeout", DefaultTimeout)
	v.SetDefault("kubelet|retries", DefaultRetries)
	v.SetDefault("kubelet|scraperMaxReruns", DefaultScraperMaxReruns)
	v.SetDefault("kubelet|fetchPodsFromKubeService", false)

	v.SetDefault("controlPlane|timeout", DefaultTimeout)
	v.SetDefault("controlPlane|retries", DefaultRetries)

	v.SetDefault("ksm|timeout", DefaultTimeout)
	v.SetDefault("ksm|retries", DefaultRetries)

	v.SetDefault("ksm|discovery|backoffDelay", 7*time.Second)
	v.SetDefault("ksm|discovery|timeout", 60*time.Second)
	v.SetDefault("ksm|enableResourceQuotaSamples", false)

	v.SetEnvPrefix("NRI_KUBERNETES")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("|", "_"))

	// Config File
	v.AddConfigPath(filePath)
	v.AddConfigPath(".")
	v.SetConfigName(fileName)

	// This could fail not only if file has not been found or has errors in the YAML/missing attributes but also with errors in environment variables.
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, err
	}

	if err := checkNamespaceSelectorConfig(cfg); err != nil {
		return &cfg, err
	}

	return &cfg, nil
}

var (
	ErrInvalidMatchExpressionsValue = errors.New("invalid matchExpressions value")
	ErrInvalidMatchLabelsValue      = errors.New("invalid matchLabels value")
)

func checkNamespaceSelectorConfig(c Config) error {
	if c.NamespaceSelector == nil {
		return nil
	}

	if c.NamespaceSelector.MatchLabels != nil {
		for _, v := range c.NamespaceSelector.MatchLabels {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%w: %v, type %T", ErrInvalidMatchLabelsValue, v, v)
			}
		}
	}

	if c.NamespaceSelector.MatchExpressions != nil {
		for _, expression := range c.NamespaceSelector.MatchExpressions {
			for _, v := range expression.Values {
				if _, ok := v.(string); !ok {
					return fmt.Errorf("%w: %v, type %T", ErrInvalidMatchExpressionsValue, v, v)
				}
			}
		}
	}

	return nil
}
