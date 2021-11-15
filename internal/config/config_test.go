package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	const testKSMHostEnvVar = "localhost:6533"
	err := os.Setenv("KSM_DISCOVERY_STATIC_URL", testKSMHostEnvVar)
	require.NoError(t, err)

	config, err := LoadConfig("testdata", "config")
	require.NoError(t, err)

	expectedConfig := Config{
		Verbose:          true,
		ClusterName:      "dummy_cluster",
		Interval:         15,
		Timeout:          30,
		CacheDir:         "/var/cache/nr-kubernetes",
		NetworkRouteFile: "/path/to/file",
		Kubelet: Kubelet{
			Discovery: struct {
				Static struct {
					URL string
				} `mapstructure:"static"`
				ClientAuthentication struct {
					CAFile      string `mapstructure:"ca_file"`
					BearerToken string `mapstructure:"bearer_token"`
				} `mapstructure:"client_authentication"`
			}{
				Static: struct {
					URL string
				}{URL: "http://localhost:8181"},
				ClientAuthentication: struct {
					CAFile      string `mapstructure:"ca_file"`
					BearerToken string `mapstructure:"bearer_token"`
				}{CAFile: "/CN=jbeda/O=app1/O=app2", BearerToken: "Bearer D45C23AB3322"},
			},
		},
		Etcd: Etcd{
			Enable: struct {
				NodeSelector string `mapstructure:"node_selector"`
			}{
				NodeSelector: "node-role=controller",
			},
			Discovery: struct {
				Static struct {
					URLs []string `mapstructure:"urls"`
				} `mapstructure:"static"`
				LocalPod struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				} `mapstructure:"local_pod"`
			}{
				Static: struct {
					URLs []string `mapstructure:"urls"`
				}{
					URLs: []string{"https://localhost:10222", "http://localhost:8080"},
				},
				LocalPod: struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				}{
					LabelSelectors: []string{"k8s-app=etcd-manager-main", "tier=control-plane,component=etcd"},
				},
			},
			Secret: struct {
				Name      string `mapstructure:"name"`
				Namespace string `mapstructure:"namespace"`
			}{Name: "nri-bundle-nri-kube-events-config", Namespace: "newrelic"},
		},
		Scheduler: Scheduler{
			Discovery: struct {
				Static struct {
					URL string `mapstructure:"url"`
				} `mapstructure:"static"`
				LocalPod struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				} `mapstructure:"local_pod"`
			}{
				Static: struct {
					URL string `mapstructure:"url"`
				}{
					URL: "https://localhost:2221",
				},
				LocalPod: struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				}{
					LabelSelectors: []string{"tier=control-plane,component=kube-scheduler"},
				},
			},
		},
		ControllerManager: ControllerManager{
			Discovery: struct {
				Static struct {
					URL string `mapstructure:"url"`
				} `mapstructure:"static"`
				LocalPod struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				} `mapstructure:"local_pod"`
			}{
				Static: struct {
					URL string `mapstructure:"url"`
				}{
					URL: "https://localhost:2223",
				},
				LocalPod: struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				}{
					LabelSelectors: []string{"tier=control-plane,component=kube-controller-manager"},
				},
			},
		},
		ApiServer: ApiServer{
			Discovery: struct {
				Static struct {
					URL string `mapstructure:"url"`
				} `mapstructure:"static"`
				LocalPod struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				} `mapstructure:"local_pod"`
			}{
				Static: struct {
					URL string `mapstructure:"url"`
				}{
					URL: "https://localhost:2226",
				},
				LocalPod: struct {
					LabelSelectors []string `mapstructure:"label_selectors"`
				}{
					LabelSelectors: []string{"tier=control-plane,component=kube-apiserver"},
				},
			},
			SecurePort: 90,
			Cache: struct {
				TTL                 string `mapstructure:"ttl"`
				TTL_jitter          int    `mapstructure:"ttl_jitter"`
				K8sVersionTTL       string `mapstructure:"k8s_version_ttl"`
				K8sVersionTTLJitter int    `mapstructure:"k8s_version_ttl_jitter"`
			}{TTL: "1h", TTL_jitter: 75, K8sVersionTTL: "3h", K8sVersionTTLJitter: 25},
		},
		KSM: KSM{
			Discovery: struct {
				Scheme      string `mapstructure:"scheme"`
				Port        int    `mapstructure:"port"`
				Distributed bool   `mapstructure:"distributed"`
				Static      struct {
					URL string `mapstructure:"url"`
				} `mapstructure:"static"`
				Endpoints struct {
					LabelSelector string `mapstructure:"label_selector"`
					Namespace     string `mapstructure:"namespace"`
				} `mapstructure:"endpoints"`
			}{
				Scheme:      "http",
				Port:        80,
				Distributed: true,
				Static: struct {
					URL string `mapstructure:"url"`
				}{URL: testKSMHostEnvVar},
				Endpoints: struct {
					LabelSelector string `mapstructure:"label_selector"`
					Namespace     string `mapstructure:"namespace"`
				}{LabelSelector: "app=ksm", Namespace: "ksm"},
			},
		},
	}

	require.Equal(t, expectedConfig, config)
}
