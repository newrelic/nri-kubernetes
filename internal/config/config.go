package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultFileName = "nri-kubernetes"
	DefaultFilePath = "/etc/newrelic-infra"
	DefaultTimeout  = 10000 * time.Millisecond
	DefaultRetries  = 4
)

type Config struct {
	Verbose        bool          `mapstructure:"verbose"`
	LogLevel       string        `mapstructure:"logLevel"`
	ClusterName    string        `mapstructure:"clusterName"`
	KubeconfigPath string        `mapstructure:"kubeconfigPath"`
	NodeIP         string        `mapstructure:"nodeIP"`
	NodeName       string        `mapstructure:"nodeName"`
	Interval       time.Duration `mapstructure:"interval"`

	Sink struct {
		HTTP HTTPSink `mapstructure:"http"`
	} `mapstructure:"sink"`

	ControlPlane `mapstructure:"controlPlane"`
	Kubelet      `mapstructure:"kubelet"`
	KSM          `mapstructure:"ksm"`
}

type HTTPSink struct {
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
	Retries int           `mapstructure:"retries"`
}

type KSM struct {
	Enabled     bool          `mapstructure:"enabled"`
	StaticURL   string        `mapstructure:"staticURL"`
	Scheme      string        `mapstructure:"scheme"`
	Port        int           `mapstructure:"port"`
	Selector    string        `mapstructure:"selector"`
	Namespace   string        `mapstructure:"namespace"`
	Distributed bool          `mapstructure:"distributed"`
	Timeout     time.Duration `mapstructure:"timeout"`
	Retries     int           `mapstructure:"retries"`
	Discovery   struct {
		BackoffDelay time.Duration `mapstructure:"backoffDelay"` // Wait BackoffDelay between discovery attempts.
		Timeout      time.Duration `mapstructure:"timeout"`      // Give up discovery and fail if Timeout has passed since first attempt.
	} `mapstructure:"discovery"`
}

type Kubelet struct {
	Enabled          bool          `mapstructure:"enabled"`
	Port             int32         `mapstructure:"port"`
	Scheme           string        `mapstructure:"scheme"`
	NetworkRouteFile string        `mapstructure:"networkRouteFile"`
	Timeout          time.Duration `mapstructure:"timeout"`
	Retries          int           `mapstructure:"retries"`
}

type ControlPlane struct {
	Enabled           bool                  `mapstructure:"enabled"`
	ETCD              ControlPlaneComponent `mapstructure:"etcd"`
	APIServer         ControlPlaneComponent `mapstructure:"apiServer"`
	ControllerManager ControlPlaneComponent `mapstructure:"controllerManager"`
	Scheduler         ControlPlaneComponent `mapstructure:"scheduler"`
	Timeout           time.Duration         `mapstructure:"timeout"`
	Retries           int                   `mapstructure:"retries"`
}

type ControlPlaneComponent struct {
	Enabled        bool                       `mapstructure:"enabled"`
	StaticEndpoint *Endpoint                  `mapstructure:"staticEndpoint"`
	Autodiscover   []AutodiscoverControlPlane `mapstructure:"autodiscover"`
}

type AutodiscoverControlPlane struct {
	Namespace string     `mapstructure:"namespace"`
	Selector  string     `mapstructure:"selector"`
	MatchNode bool       `mapstructure:"matchNode"`
	Endpoints []Endpoint `mapstructure:"endpoints"`
}

type Endpoint struct {
	URL                string `mapstructure:"url"`
	Auth               *Auth  `mapstructure:"auth"`
	InsecureSkipVerify bool   `mapstructure:"insecureSkipVerify"`
}

type Auth struct {
	Type string `mapstructure:"type"`
	MTLS *MTLS  `mapstructure:"mtls"`
}

type MTLS struct {
	TLSSecretName      string `mapstructure:"secretName"`
	TLSSecretNamespace string `mapstructure:"secretNamespace"`
}

func LoadConfig(filePath string, fileName string) (*Config, error) {
	v := viper.New()

	// We need to assure that defaults have been set in order to bind env variables.
	// https://github.com/spf13/viper/issues/584
	v.SetDefault("clusterName", "cluster")
	v.SetDefault("verbose", false)
	v.SetDefault("kubelet.networkRouteFile", "/proc/net/route")
	v.SetDefault("nodeName", "node")
	v.SetDefault("nodeIP", "node")

	// Sane connection defaults
	v.SetDefault("sink.http.port", 0)
	v.SetDefault("sink.http.timeout", DefaultTimeout)
	v.SetDefault("sink.http.retries", DefaultRetries)

	v.SetDefault("kubelet.timeout", DefaultTimeout)
	v.SetDefault("kubelet.retries", DefaultRetries)

	v.SetDefault("controlPlane.timeout", DefaultTimeout)
	v.SetDefault("controlPlane.retries", DefaultRetries)

	v.SetDefault("ksm.timeout", DefaultTimeout)
	v.SetDefault("ksm.retries", DefaultRetries)

	v.SetDefault("ksm.discovery.backoffDelay", 7*time.Second)
	v.SetDefault("ksm.discovery.timeout", 60*time.Second)

	v.SetEnvPrefix("NRI_KUBERNETES")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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

	return &cfg, nil
}
