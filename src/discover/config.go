package discover

import (
	"strings"
	"time"

	internalconfig "github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/spf13/viper"
)

const (
	DefaultConfigFileName   = "nri-k8s-discover"
	DefaultConfigFolderName = "/etc/newrelic-infra"

	// DefaultInterval is longer than the main agent's 15s; discovery data changes rarely.
	DefaultInterval = 60 * time.Second
)

// Config holds the configuration for the discovery agent.
// It is intentionally minimal — no KSM, Kubelet, or ControlPlane sections.
type Config struct {
	Verbose        bool          `mapstructure:"verbose"`
	LogLevel       string        `mapstructure:"logLevel"`
	ClusterName    string        `mapstructure:"clusterName"`
	KubeconfigPath string        `mapstructure:"kubeconfigPath"`
	Interval       time.Duration `mapstructure:"interval"`

	Sink struct {
		Type string                 `mapstructure:"type"`
		HTTP internalconfig.HTTPSink `mapstructure:"http"`
	} `mapstructure:"sink"`

	NamespaceSelector *internalconfig.NamespaceSelector `mapstructure:"namespaceSelector"`

	// NRNamespace is the Kubernetes namespace where the New Relic bundle is deployed.
	// Used to look up OHI ConfigMaps for instrumentation detection.
	// Defaults to "newrelic".
	NRNamespace string `mapstructure:"nrNamespace"`
}

// LoadConfig loads the discovery agent config from disk and environment variables.
// It mirrors the pattern from internal/config/config.go.
func LoadConfig(filePath, fileName string) (*Config, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("|"))

	v.SetDefault(metricClusterName, "cluster")
	v.SetDefault("verbose", false)
	v.SetDefault("interval", DefaultInterval)

	v.SetDefault("sink|type", internalconfig.SinkTypeHTTP)
	v.SetDefault("sink|http|port", 0)
	v.SetDefault("sink|http|timeout", internalconfig.DefaultAgentTimeout)
	v.SetDefault("sink|http|retries", internalconfig.DefaultRetries)
	v.SetDefault("sink|http|probeTimeout", internalconfig.DefaultProbeTimeout)
	v.SetDefault("sink|http|probeBackoff", internalconfig.DefaultProbeBackoff)

	v.SetDefault("nrNamespace", "newrelic")

	v.SetEnvPrefix("NRI_K8S_DISCOVER")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("|", "_"))

	v.AddConfigPath(filePath)
	v.AddConfigPath(".")
	v.SetConfigName(fileName)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
