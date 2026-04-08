package metric

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen,maintidx // Test requires comprehensive metric coverage.
func TestKubeletMetricsFetchFunc(t *testing.T) {
	t.Parallel()

	// Mock prometheus metric families for kubelet health metrics.
	mockFetcher := func(_ []prometheus.Query) ([]prometheus.MetricFamily, error) {
		return []prometheus.MetricFamily{
			// PLEG metrics
			{
				Name: "kubelet_pleg_relist_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"quantile": "0.5"},
						Value:  prometheus.GaugeValue(0.001),
					},
					{
						Labels: prometheus.Labels{"quantile": "0.99"},
						Value:  prometheus.GaugeValue(0.5), // 500ms - this should be flagged
					},
				},
			},
			{
				Name: "kubelet_pleg_relist_interval_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"quantile": "0.99"},
						Value:  prometheus.GaugeValue(1.2),
					},
				},
			},
			// Pod lifecycle metrics
			{
				Name: "kubelet_pod_start_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"quantile": "0.99"},
						Value:  prometheus.GaugeValue(3.5),
					},
				},
			},
			{
				Name: "kubelet_pod_worker_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"quantile": "0.99"},
						Value:  prometheus.GaugeValue(0.05),
					},
				},
			},
			// Runtime operations
			{
				Name: "kubelet_runtime_operations_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"operation_type": "create_container",
							"quantile":       "0.99",
						},
						Value: prometheus.GaugeValue(0.1),
					},
					{
						Labels: prometheus.Labels{
							"operation_type": "start_container",
							"quantile":       "0.99",
						},
						Value: prometheus.GaugeValue(0.05),
					},
				},
			},
			{
				Name: "kubelet_runtime_operations_errors_total",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"operation_type": "create_container"},
						Value:  prometheus.CounterValue(5),
					},
					{
						Labels: prometheus.Labels{"operation_type": "start_container"},
						Value:  prometheus.CounterValue(2),
					},
				},
			},
			{
				Name: "kubelet_runtime_operations_total",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"operation_type": "create_container"},
						Value:  prometheus.CounterValue(1000),
					},
					{
						Labels: prometheus.Labels{"operation_type": "start_container"},
						Value:  prometheus.CounterValue(1000),
					},
				},
			},
			// Image operations
			{
				Name: "kubelet_image_pull_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"quantile": "0.99"},
						Value:  prometheus.GaugeValue(15.0),
					},
				},
			},
			// Volume operations
			{
				Name: "storage_operation_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"operation_name": "volume_mount",
							"volume_plugin":  "kubernetes.io/csi",
							"quantile":       "0.99",
						},
						Value: prometheus.GaugeValue(0.8),
					},
					{
						Labels: prometheus.Labels{
							"operation_name": "volume_unmount",
							"volume_plugin":  "kubernetes.io/csi",
							"quantile":       "0.99",
						},
						Value: prometheus.GaugeValue(0.3),
					},
				},
			},
			{
				Name: "storage_operation_errors_total",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"operation_name": "volume_mount"},
						Value:  prometheus.CounterValue(3),
					},
				},
			},
			// Evictions
			{
				Name: "kubelet_evictions_total",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"eviction_signal": "memory.available"},
						Value:  prometheus.CounterValue(2),
					},
					{
						Labels: prometheus.Labels{"eviction_signal": "nodefs.available"},
						Value:  prometheus.CounterValue(1),
					},
				},
			},
			// HTTP requests
			{
				Name: "kubelet_http_requests_total",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"method": "GET",
							"path":   "/metrics",
						},
						Value: prometheus.CounterValue(1000),
					},
					{
						Labels: prometheus.Labels{
							"method": "GET",
							"path":   "/stats/summary",
						},
						Value: prometheus.CounterValue(500),
					},
					{
						Labels: prometheus.Labels{
							"method": "POST",
							"path":   "/api/v1/pods",
						},
						Value: prometheus.CounterValue(100),
					},
				},
			},
			{
				Name: "kubelet_http_requests_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"method":   "GET",
							"quantile": "0.99",
						},
						Value: prometheus.GaugeValue(0.02),
					},
					{
						Labels: prometheus.Labels{
							"method":   "POST",
							"quantile": "0.99",
						},
						Value: prometheus.GaugeValue(0.05),
					},
				},
			},
			// Node stats
			{
				Name: "kubelet_running_pods",
				Metrics: []prometheus.Metric{
					{Value: prometheus.GaugeValue(25)},
				},
			},
			{
				Name: "kubelet_running_containers",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"container_state": "running"},
						Value:  prometheus.CounterValue(75),
					},
					{
						Labels: prometheus.Labels{"container_state": "exited"},
						Value:  prometheus.CounterValue(10),
					},
				},
			},
			// Node name
			{
				Name: "kubelet_node_name",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{"node": "test-node-1"},
						Value:  prometheus.CounterValue(1),
					},
				},
			},
			// Cgroup manager
			{
				Name: "kubelet_cgroup_manager_duration_seconds",
				Metrics: []prometheus.Metric{
					{
						Labels: prometheus.Labels{
							"operation_type": "create",
							"quantile":       "0.99",
						},
						Value: prometheus.GaugeValue(0.01),
					},
				},
			},
		}, nil
	}

	// Call the fetch function
	fetchFunc := KubeletMetricsFetchFunc(mockFetcher, "test-node")
	rawGroups, err := fetchFunc()
	require.NoError(t, err)

	// Verify structure
	require.NotNil(t, rawGroups)
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok, "node group should exist")

	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok, "test-node should exist in node group")

	// Verify PLEG metrics (critical for kubelet health)
	assert.Equal(t, prometheus.GaugeValue(0.5), nodeMetrics["kubeletPLEGRelistDurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(1.2), nodeMetrics["kubeletPLEGRelistIntervalSeconds"])

	// Verify pod lifecycle metrics
	assert.Equal(t, prometheus.GaugeValue(3.5), nodeMetrics["kubeletPodStartDurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(0.05), nodeMetrics["kubeletPodWorkerDurationSeconds"])

	// Verify runtime operations
	assert.Equal(t, prometheus.GaugeValue(0.1), nodeMetrics["kubeletRuntimeOperation_create_container_DurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(0.05), nodeMetrics["kubeletRuntimeOperation_start_container_DurationSeconds"])
	assert.Equal(t, prometheus.CounterValue(5), nodeMetrics["kubeletRuntimeOperation_create_container_ErrorsTotal"])
	assert.Equal(t, prometheus.CounterValue(2), nodeMetrics["kubeletRuntimeOperation_start_container_ErrorsTotal"])
	assert.Equal(t, prometheus.CounterValue(1000), nodeMetrics["kubeletRuntimeOperation_create_container_Total"])
	assert.Equal(t, prometheus.CounterValue(1000), nodeMetrics["kubeletRuntimeOperation_start_container_Total"])

	// Verify image operations
	assert.Equal(t, prometheus.GaugeValue(15.0), nodeMetrics["kubeletImagePullDurationSeconds"])

	// Verify volume operations
	assert.Equal(t, prometheus.GaugeValue(0.8), nodeMetrics["kubeletStorageOperation_volume_mount_DurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(0.8), nodeMetrics["kubeletStorageOperation_volume_mount_kubernetes.io/csi_DurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(0.3), nodeMetrics["kubeletStorageOperation_volume_unmount_DurationSeconds"])
	assert.Equal(t, prometheus.CounterValue(3), nodeMetrics["kubeletStorageOperation_volume_mount_ErrorsTotal"])

	// Verify evictions
	assert.Equal(t, prometheus.CounterValue(2), nodeMetrics["kubeletEvictions_memory.available_Total"])
	assert.Equal(t, prometheus.CounterValue(1), nodeMetrics["kubeletEvictions_nodefs.available_Total"])

	// Verify HTTP requests (last value per method)
	assert.Equal(t, prometheus.CounterValue(500), nodeMetrics["kubeletHTTPRequests_GET_Total"]) // Last GET value
	assert.Equal(t, prometheus.CounterValue(100), nodeMetrics["kubeletHTTPRequests_POST_Total"])
	assert.Equal(t, prometheus.GaugeValue(0.02), nodeMetrics["kubeletHTTPRequests_GET_DurationSeconds"])
	assert.Equal(t, prometheus.GaugeValue(0.05), nodeMetrics["kubeletHTTPRequests_POST_DurationSeconds"])

	// Verify node stats
	assert.Equal(t, prometheus.GaugeValue(25), nodeMetrics["kubeletRunningPods"])
	assert.Equal(t, prometheus.CounterValue(75), nodeMetrics["kubeletRunningContainers_running"])
	assert.Equal(t, prometheus.CounterValue(10), nodeMetrics["kubeletRunningContainers_exited"])

	// Verify node name
	assert.Equal(t, "test-node-1", nodeMetrics["kubeletNodeNameMetric"])

	// Verify cgroup manager
	assert.Equal(t, prometheus.GaugeValue(0.01), nodeMetrics["kubeletCgroupManager_create_DurationSeconds"])
}

func TestKubeletMetricsFetchFunc_EmptyMetrics(t *testing.T) {
	t.Parallel()

	// Mock fetcher that returns empty metrics.
	mockFetcher := func(_ []prometheus.Query) ([]prometheus.MetricFamily, error) {
		return []prometheus.MetricFamily{}, nil
	}

	fetchFunc := KubeletMetricsFetchFunc(mockFetcher, "test-node")
	rawGroups, err := fetchFunc()
	require.NoError(t, err)

	// Should still have a node group with the node
	require.NotNil(t, rawGroups)
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok)

	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok)

	// Metrics should be empty
	assert.Empty(t, nodeMetrics)
}

func TestKubeletHealthQueries(t *testing.T) {
	t.Parallel()

	// Verify that all important queries are present.
	queryNames := make(map[string]bool)
	for _, query := range GetKubeletHealthQueries() {
		queryNames[query.MetricName] = true
	}

	// Critical metrics that should be queried
	criticalMetrics := []string{
		"kubelet_pleg_relist_duration_seconds",
		"kubelet_pleg_relist_interval_seconds",
		"kubelet_pod_start_duration_seconds",
		"kubelet_runtime_operations_duration_seconds",
		"kubelet_runtime_operations_errors_total",
		"kubelet_evictions_total",
		"storage_operation_duration_seconds",
		"kubelet_running_pods",
	}

	for _, metric := range criticalMetrics {
		assert.True(t, queryNames[metric], "Critical metric %s should be in queries", metric)
	}
}
