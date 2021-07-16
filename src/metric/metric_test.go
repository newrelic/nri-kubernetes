package metric

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

func TestK8sClusterMetricsManipulator(t *testing.T) {
	metadata := &integration.EntityMetadata{
		Name:      "fluentd-elasticsearch-jnqb7",
		Namespace: "k8s:playground:kube-system:pod",
	}
	metricSet := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":        "K8sPodSample",
			"podInfo.namespace": "kube-system",
			"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
			"displayName":       "fluentd-elasticsearch-jnqb7",
			"entityName":        "k8s:playground:kube-system:pod:fluentd-elasticsearch-jnqb7",
			"clusterName":       "playground",
		},
	}

	err := K8sClusterMetricsManipulator(metricSet, metadata, "modifiedClusterName")
	assert.Nil(t, err)

	expectedMetricSet := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":        "K8sPodSample",
			"podInfo.namespace": "kube-system",
			"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
			"displayName":       "fluentd-elasticsearch-jnqb7",
			"entityName":        "k8s:playground:kube-system:pod:fluentd-elasticsearch-jnqb7",
			"clusterName":       "modifiedClusterName",
		},
	}
	assert.Equal(t, expectedMetricSet, metricSet)
}

func TestK8sMetricSetTypeGuesser(t *testing.T) {
	testCases := []struct {
		groupLabel string
		expected   string
	}{
		{groupLabel: "replicaset", expected: "K8sReplicasetSample"},
		{groupLabel: "api-server", expected: "K8sApiServerSample"},
		{groupLabel: "controller-manager", expected: "K8sControllerManagerSample"},
		{groupLabel: "-controller-manager-", expected: "K8sControllerManagerSample"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.groupLabel, func(*testing.T) {
			guess, err := K8sMetricSetTypeGuesser("", testCase.groupLabel, "", nil)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expected, guess)
		})
	}
}

func TestK8sEntityMetricsManipulator(t *testing.T) {
	metadata := &integration.EntityMetadata{
		Name:      "fluentd-elasticsearch-jnqb7",
		Namespace: "k8s:playground:kube-system:pod",
	}
	metricSet := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":        "K8sPodSample",
			"podInfo.namespace": "kube-system",
			"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
			"entityName":        "fluentd-elasticsearch-jnqb7",
			"clusterName":       "playground",
		},
	}

	err := K8sEntityMetricsManipulator(metricSet, metadata, "")
	assert.Nil(t, err)

	expectedMetricSet := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":        "K8sPodSample",
			"podInfo.namespace": "kube-system",
			"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
			"displayName":       "fluentd-elasticsearch-jnqb7",
			"entityName":        "fluentd-elasticsearch-jnqb7",
			"clusterName":       "playground",
		},
	}
	assert.Equal(t, expectedMetricSet, metricSet)
}

func TestSubtractorFunc(t *testing.T) {
	// given 2 FetchFunc
	var left definition.FetchFunc
	var right definition.FetchFunc
	left = func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return float64(10), nil
	}
	right = func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return float64(5), nil
	}

	sub := Subtract(left, right)
	result, err := sub("", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, float64(5), result)
}
