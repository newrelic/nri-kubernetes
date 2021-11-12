package config

import (
	"os"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/octago/sflags"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Verbose     bool          `mapstructure:"verbose"`
	ClusterName string        `mapstructure:"cluster_name"`
	Interval    time.Duration `mapstructure:"interval"`

	KSM       `mapstructure:"ksm"`
	Kubelet   `mapstructure:"kubelet"`
	Etcd      `mapstructure:"etcd"`
	ApiServer `mapstructure:"api_server"`

	Logger log.Logger
}

type Kubelet struct {
}

type Etcd struct {
}

type ApiServer struct {
}

type KSM struct {
	// URL defines a static endpoint for KSM.
	StaticEndpoint string `mapstructure:"static_endpoint"`

	// Autodiscovery settings.
	// Scheme that will be used to hit the endpoints of discovered KSM services. Defaults to http.
	Scheme string `mapstructure:"scheme"`
	// If set, Port will discard all endpoints discovered that do not use this specified port. Otherwise, all endpoints will be considered.
	Port int `mapstructure:"port"`
	// PodLabel is the selector used to filter Endpoints.
	PodLabel string `mapstructure:"pod_label"`
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string `mapstructure:"namespace"`
	// If set, Distributed will instruct the integration to scrape all KSM endpoints rather than just the first one.
	Distributed bool `mapstructure:"distributed"`
}

func LoadConfig() (Config, error) {
	var cfg Config

	// Env Variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Flags
	flagSet := pflag.NewFlagSet("server_a", pflag.ContinueOnError)
	if err := gpflag.ParseTo(&cfg, flagSet, sflags.FlagDivider("."), sflags.FlagTag("mapstructure")); err != nil {
		return Config{}, err
	}
	flagSet.Parse(os.Args[1:])
	if err := viper.BindPFlags(flagSet); err != nil {
		return Config{}, err
	}

	// Config File
	viper.SetConfigName("server")
	viper.AddConfigPath("testdata")

	var isFileRead bool
	if err := viper.ReadInConfig(); err == nil {
		isFileRead = true
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	cfg.Logger = log.NewStdErr(cfg.Verbose)
	if isFileRead {
		cfg.Logger.Infof("Using config file: %s \n", viper.ConfigFileUsed())
	}

	return cfg, nil
}

/*
func LoadConfig() Config {
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

	return Config{
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
}*/
