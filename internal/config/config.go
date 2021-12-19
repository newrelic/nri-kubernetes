package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	FileName = "nri-kubernetes-config"
	FilePath = "/etc/newrelic-infra/integrations.d/"
)

type Config struct {
	Verbose          bool          `mapstructure:"verbose"`
	ClusterName      string        `mapstructure:"clusterName"`
	NetworkRouteFile string        `mapstructure:"networkRouteFile"`
	KubeconfigPath   string        `mapstructure:"kubeconfigPath"`
	NodeIP           string        `mapstructure:"nodeIP"`
	NodeName         string        `mapstructure:"nodeName"`
	HTTPServerPort   string        `mapstructure:"httpServerPort"`
	Interval         time.Duration `mapstructure:"interval"`
	Timeout          time.Duration `mapstructure:"timeout"`

	ControlPlane `mapstructure:"controlPlane"`
	Kubelet      `mapstructure:"kubelet"`
	KSM          `mapstructure:"ksm"`
}

type KSM struct {
	StaticURL   string `mapstructure:"staticURL"`
	Scheme      string `mapstructure:"scheme"`
	Port        int    `mapstructure:"port"`
	Selector    string `mapstructure:"selector"`
	Namespace   string `mapstructure:"namespace"`
	Distributed bool   `mapstructure:"distributed"`
	Enabled     bool   `mapstructure:"enabled"`
}

type Kubelet struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int32  `mapstructure:"port"`
	Scheme  string `mapstructure:"scheme"`
}

type ControlPlane struct {
	Enabled           bool                  `mapstructure:"enabled"`
	ETCD              ControlPlaneComponent `mapstructure:"etcd"`
	APIServer         ControlPlaneComponent `mapstructure:"apiServer"`
	ControllerManager ControlPlaneComponent `mapstructure:"controllerManager"`
	Scheduler         ControlPlaneComponent `mapstructure:"scheduler"`
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

func (m MTLS) SecretNamespace() string {
	if m.TLSSecretNamespace == "" {
		return "default"
	}

	return m.TLSSecretNamespace
}

func LoadConfig(filePath string, fileName string) (*Config, error) {
	v := viper.New()

	// We need to assure that defaults have been set in order to bind env variables.
	// https://github.com/spf13/viper/issues/584
	v.SetDefault("clusterName", "cluster")
	v.SetDefault("nodeName", "node")
	v.SetDefault("nodeIP", "node")
	v.SetDefault("httpServerPort", 0)
	v.SetDefault("controlPlane.enabled", false)
	v.SetDefault("ksm.enabled", false)
	v.SetDefault("kubelet.enabled", false)

	v.SetEnvPrefix("NRI_KUBERNETES")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Config File
	v.AddConfigPath(filePath)
	v.SetConfigName(fileName)

	// If error reading file or file not found, use flag/env variables
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
