package metric

import (
	"fmt"
	"testing"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/stretchr/testify/assert"
)

var rawGroups = definition.RawGroups{
	"pod": {
		"fluentd-elasticsearch-jnqb7": definition.RawMetrics{
			"kube_pod_start_time": prometheus.Metric{
				Value: prometheus.GaugeValue(1507117436),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "fluentd-elasticsearch-jnqb7",
				},
			},
			"kube_pod_info": prometheus.Metric{
				Value: prometheus.GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "ReplicaSet",
					"created_by_name": "fluentd-elasticsearch-fafnoa",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "fluentd-elasticsearch-jnqb7",
				},
			},
		},
		"newrelic-infra-monitoring-cglrn": definition.RawMetrics{
			"kube_pod_start_time": prometheus.Metric{
				Value: prometheus.GaugeValue(1510579152),
				Labels: map[string]string{
					"namespace": "kube-system",
					"pod":       "newrelic-infra-monitoring-cglrn",
				},
			},
			"kube_pod_info": prometheus.Metric{
				Value: prometheus.GaugeValue(1),
				Labels: map[string]string{
					"created_by_kind": "DaemonSet",
					"created_by_name": "newrelic-infra-monitoring",
					"namespace":       "kube-system",
					"node":            "minikube",
					"pod":             "newrelic-infra-monitoring-cglrn",
				},
			},
		},
	},
}

var rawGroupWithReplicaSet = definition.RawGroups{
	"replicaset": {
		"kube-state-metrics-4044341274": definition.RawMetrics{
			"kube_replicaset_created": prometheus.Metric{
				Value: prometheus.GaugeValue(1507117436),
				Labels: map[string]string{
					"namespace":  "kube-system",
					"replicaset": "kube-state-metrics-4044341274",
				},
			},
		},
	},
}

func TestGetDeploymentNameForReplicaSet_ValidName(t *testing.T) {
	expectedValue := "kube-state-metrics"
	fetchedValue, err := GetDeploymentNameForReplicaSet()("replicaset", "kube-state-metrics-4044341274", rawGroupWithReplicaSet)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestGetDeploymentNameForReplicaSet_ErrorOnEmptyData(t *testing.T) {
	raw := definition.RawGroups{
		"replicaset": {
			"kube-state-metrics-4044341274": definition.RawMetrics{
				"kube_replicaset_created": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"namespace":  "kube-system",
						"replicaset": "",
					},
				},
			},
		},
	}
	fetchedValue, err := GetDeploymentNameForReplicaSet()("replicaset", "kube-state-metrics-4044341274", raw)
	assert.EqualError(t, err, "error generating deployment name for replica set. replicaset field is empty")
	assert.Empty(t, fetchedValue)
}

func TestGetDeploymentNameForPod_CreatedByReplicaSet(t *testing.T) {
	expectedValue := "fluentd-elasticsearch"
	fetchedValue, err := GetDeploymentNameForPod()("pod", "fluentd-elasticsearch-jnqb7", rawGroups)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestGetDeploymentNameForPod_NotCreatedByReplicaSet(t *testing.T) {
	rawEntityID := "kube-addon-manager-minikube"
	raw := definition.RawGroups{
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"created_by_kind": "<none>",
						"created_by_name": "<none>",
					},
				},
			},
		},
	}

	fetchedValue, err := GetDeploymentNameForPod()("pod", rawEntityID, raw)
	assert.Nil(t, err)
	assert.Empty(t, fetchedValue)
}

func TestGetDeploymentNameForPod_ErrorOnEmptyData(t *testing.T) {
	rawEntityID := "kube-addon-manager-minikube"
	raw := definition.RawGroups{
		"pod": {
			"kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"created_by_name": "newrelic-infra-monitoring",
						"created_by_kind": "", // Empty created_by_kind
					},
				},
			},
		},
	}

	fetchedValue, err := GetDeploymentNameForPod()("pod", rawEntityID, raw)
	assert.EqualError(t, err, "error generating deployment name for pod. created_by_kind field is empty")
	assert.Empty(t, fetchedValue)

	// Empty created_by_name
	m := raw["pod"]["kube-addon-manager-minikube"]["kube_pod_info"].(prometheus.Metric)
	m.Labels = map[string]string{
		"created_by_name": "",
		"created_by_kind": "DaemonSet",
	}

	raw["pod"]["kube-addon-manager-minikube"]["kube_pod_info"] = m

	fetchedValue, err = GetDeploymentNameForPod()("pod", rawEntityID, raw)
	assert.EqualError(t, err, "error generating deployment name for pod. created_by_name field is empty")
	assert.Empty(t, fetchedValue)
}

func TestGetDeploymentNameForContainer_CreatedByReplicaSet(t *testing.T) {
	expectedValue := "fluentd-elasticsearch"
	podRawID := "kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics"
	raw := definition.RawGroups{
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"created_by_kind": "ReplicaSet",
						"created_by_name": "fluentd-elasticsearch-fafnoa",
						"namespace":       "kube-system",
						"node":            "minikube",
						"pod":             "kube-addon-manager-minikube",
					},
				},
			},
		},
		"container": {
			podRawID: definition.RawMetrics{
				"kube_pod_container_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "kube-addon-manager-minikube",
					},
				},
			},
		},
	}
	fetchedValue, err := GetDeploymentNameForContainer()("container", podRawID, raw)
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, fetchedValue)
}

func TestGetDeploymentNameForContainer_NotCreatedByReplicaSet(t *testing.T) {
	podRawID := "kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics"
	raw := definition.RawGroups{
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"created_by_kind": "DaemonSet",
						"created_by_name": "newrelic-infra-monitoring",
						"namespace":       "kube-system",
						"node":            "minikube",
						"pod":             "kube-addon-manager-minikube",
					},
				},
			},
		},
		"container": {
			podRawID: definition.RawMetrics{
				"kube_pod_container_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"image":     "gcr.io/google_containers/kube-state-metrics:v1.1.0",
						"namespace": "kube-system",
						"pod":       "kube-addon-manager-minikube",
					},
				},
			},
		},
	}
	fetchedValue, err := GetDeploymentNameForContainer()("container", podRawID, raw)
	assert.Nil(t, err)
	assert.Empty(t, fetchedValue)
}

func TestGetDeploymentNameForContainer_ErrorOnMissingData(t *testing.T) {
	podRawID := "kube-system_fluentd-elasticsearch-jnqb7_kube-state-metrics"
	raw := definition.RawGroups{
		"pod": {
			"kube-system_kube-addon-manager-minikube": definition.RawMetrics{
				"kube_pod_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1507117436),
					Labels: map[string]string{
						"namespace": "kube-system",
						"node":      "minikube",
						"pod":       "kube-addon-manager-minikube",
					},
				},
			},
		},
		"container": {
			podRawID: definition.RawMetrics{
				"kube_pod_container_info": prometheus.Metric{
					Value: prometheus.GaugeValue(1),
					Labels: map[string]string{
						"container": "kube-state-metrics",
						"namespace": "kube-system",
						"pod":       "kube-addon-manager-minikube",
					},
				},
			},
		},
	}

	// created_by_kind is empty
	raw["pod"]["kube-system_kube-addon-manager-minikube"]["kube_pod_info"].(prometheus.Metric).Labels["created_by_name"] = "newrelic-infra-monitoring"

	fetchedValue, err := GetDeploymentNameForContainer()("container", podRawID, raw)
	assert.EqualError(t, err, "error generating deployment name for container. created_by_kind field is missing")
	assert.Empty(t, fetchedValue)

	// created_by_name is empty
	raw["pod"]["kube-system_kube-addon-manager-minikube"]["kube_pod_info"].(prometheus.Metric).Labels["created_by_name"] = ""
	raw["pod"]["kube-system_kube-addon-manager-minikube"]["kube_pod_info"].(prometheus.Metric).Labels["created_by_kind"] = "DaemonSet"

	fetchedValue, err = GetDeploymentNameForContainer()("container", podRawID, raw)
	assert.EqualError(t, err, "error generating deployment name for container. created_by_name field is missing")
	assert.Empty(t, fetchedValue)

	// Missing created_by_kind and created_by_name
	m := raw["pod"]["kube-system_kube-addon-manager-minikube"]["kube_pod_info"].(prometheus.Metric)
	m.Labels = map[string]string{
		"namespace": "kube-system",
		"node":      "minikube",
		"pod":       "kube-addon-manager-minikube",
	}

	raw["pod"]["kube-system_kube-addon-manager-minikube"]["kube_pod_info"] = m

	fetchedValue, err = GetDeploymentNameForContainer()("container", podRawID, raw)
	assert.EqualError(t, err, "error generating deployment name for container. created_by_kind field is missing")
	assert.Empty(t, fetchedValue)
}

func TestStatusForContainer(t *testing.T) {
	var raw definition.RawGroups
	var statusTests = []struct {
		s        string
		expected string
	}{
		{"running", "Running"},
		{"terminated", "Terminated"},
		{"waiting", "Waiting"},
		{"whatever", "Unknown"},
	}

	for _, tt := range statusTests {
		raw = definition.RawGroups{
			"container": {
				"kube-addon-manager-minikube": definition.RawMetrics{
					fmt.Sprintf("kube_pod_container_status_%s", tt.s): prometheus.Metric{
						Value: prometheus.GaugeValue(1),
						Labels: map[string]string{
							"namespace": "kube-system",
						},
					},
				},
			},
		}
		actual, err := GetStatusForContainer()("container", "kube-addon-manager-minikube", raw)
		assert.Equal(t, tt.expected, actual)
		assert.NoError(t, err)
	}
}
