package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
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
