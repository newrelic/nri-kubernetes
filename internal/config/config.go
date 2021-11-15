package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultSchema = "http"
	FileName      = "nri-kubernetes"
	FilePath      = "/etc/newrelic-infra/integrations.d/"
)

type Config struct {
	Verbose          bool          `mapstructure:"verbose"`
	ClusterName      string        `mapstructure:"cluster_name"`
	Interval         time.Duration `mapstructure:"interval"`
	Timeout          time.Duration `mapstructure:"timeout"`
	CacheDir         string        `mapstructure:"cache_dir"`
	NetworkRouteFile string        `mapstructure:"network_route_file"`

	Kubelet           `mapstructure:"kubelet"`
	Etcd              `mapstructure:"etcd"`
	Scheduler         `mapstructure:"scheduler"`
	ControllerManager `mapstructure:"controller_manager"`
	APIServer         `mapstructure:"api_server"`
	KSM               `mapstructure:"ksm"`
}

type Kubelet struct {
	Discovery struct {
		Static struct {
			URL string
		} `mapstructure:"static"`
		ClientAuthentication struct {
			CAFile      string `mapstructure:"ca_file"`
			BearerToken string `mapstructure:"bearer_token"`
		} `mapstructure:"client_authentication"`
	} `mapstructure:"discovery"`
}

type Etcd struct {
	Enable struct {
		NodeSelector string `mapstructure:"node_selector"`
	} `mapstructure:"enable"`
	Discovery struct {
		Static struct {
			URLs []string `mapstructure:"urls"`
		} `mapstructure:"static"`
		LocalPod struct {
			LabelSelectors []string `mapstructure:"label_selectors"`
		} `mapstructure:"local_pod"`
	} `mapstructure:"discovery"`
	Secret struct {
		Name      string `mapstructure:"name"`
		Namespace string `mapstructure:"namespace"`
	} `mapstructure:"secret"`
}

type Scheduler struct {
	Discovery struct {
		Static struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"static"`
		LocalPod struct {
			LabelSelectors []string `mapstructure:"label_selectors"`
		} `mapstructure:"local_pod"`
	} `mapstructure:"discovery"`
}

type ControllerManager struct {
	Discovery struct {
		Static struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"static"`
		LocalPod struct {
			LabelSelectors []string `mapstructure:"label_selectors"`
		} `mapstructure:"local_pod"`
	} `mapstructure:"discovery"`
}

type APIServer struct {
	Discovery struct {
		Static struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"static"`
		LocalPod struct {
			LabelSelectors []string `mapstructure:"label_selectors"`
		} `mapstructure:"local_pod"`
	} `mapstructure:"discovery"`
	Cache struct {
		TTL                 string `mapstructure:"ttl"`
		TTLJitter           int    `mapstructure:"ttl_jitter"`
		K8sVersionTTL       string `mapstructure:"k8s_version_ttl"`
		K8sVersionTTLJitter int    `mapstructure:"k8s_version_ttl_jitter"`
	} `mapstructure:"cache"`
	SecurePort int `mapstructure:"secure_port"`
}

type KSM struct {
	Discovery struct {
		// Scheme that will be used to hit the endpoints of discovered KSM services. Defaults to http.
		Scheme string `mapstructure:"scheme"`
		// If set, Port will discard all endpoints discovered that do not use this specified port. Otherwise, all endpoints will be considered.
		Port int `mapstructure:"port"`
		// If set, Distributed will instruct the integration to scrape all KSM endpoints rather than just the first one.
		Distributed bool `mapstructure:"distributed"`
		Static      struct {
			URL string `mapstructure:"url"`
		} `mapstructure:"static"`
		Endpoints struct {
			LabelSelector string `mapstructure:"label_selector"`
			Namespace     string `mapstructure:"namespace"`
		} `mapstructure:"endpoints"`
	} `mapstructure:"discovery"`
}

func LoadConfig(configPath, configName string) (Config, error) {
	var cfg Config

	// Env Variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Config File
	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	// If error reading file or file not found, use flag/env variables
	_ = viper.ReadInConfig()
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
