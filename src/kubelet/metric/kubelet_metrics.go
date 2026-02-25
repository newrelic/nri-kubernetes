package metric

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const (
	// KubeletMetricsPath is the path where kubelet serves its own health/performance metrics
	KubeletMetricsPath = "/metrics"
)

// KubeletMetricsFetchFunc creates a FetchFunc that fetches kubelet's own health metrics
func KubeletMetricsFetchFunc(fetchAndFilterPrometheus prometheus.FetchAndFilterMetricsFamilies, nodeName string) data.FetchFunc {
	return func() (definition.RawGroups, error) {
		families, err := fetchAndFilterPrometheus(kubeletHealthQueries)
		if err != nil {
			return nil, fmt.Errorf("error requesting kubelet metrics endpoint: %w", err)
		}

		g := definition.RawGroups{
			"node": {
				nodeName: make(definition.RawMetrics),
			},
		}

		nodeMetrics := g["node"][nodeName]

		for _, family := range families {
			switch family.Name {
			// PLEG (Pod Lifecycle Event Generator) metrics - critical for kubelet health
			case "kubelet_pleg_relist_duration_seconds":
				// Get the quantile=0.99 value (99th percentile)
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletPLEGRelistDurationSeconds"] = m.Value
						break
					}
				}
			case "kubelet_pleg_relist_interval_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletPLEGRelistIntervalSeconds"] = m.Value
						break
					}
				}

			// Pod lifecycle metrics
			case "kubelet_pod_start_duration_seconds":
				// Get the quantile=0.99 value
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletPodStartDurationSeconds"] = m.Value
						break
					}
				}
			case "kubelet_pod_worker_duration_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletPodWorkerDurationSeconds"] = m.Value
						break
					}
				}

			// Runtime operations
			case "kubelet_runtime_operations_duration_seconds":
				// Get the quantile=0.99 value
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						operationType := m.Labels["operation_type"]
						if operationType != "" {
							metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_DurationSeconds", operationType)
							nodeMetrics[metricName] = m.Value
						}
					}
				}
			case "kubelet_runtime_operations_errors_total":
				for _, m := range family.Metrics {
					operationType := m.Labels["operation_type"]
					if operationType != "" {
						metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_ErrorsTotal", operationType)
						nodeMetrics[metricName] = m.Value
					}
				}
			case "kubelet_runtime_operations_total":
				for _, m := range family.Metrics {
					operationType := m.Labels["operation_type"]
					if operationType != "" {
						metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_Total", operationType)
						nodeMetrics[metricName] = m.Value
					}
				}

			// Image management
			case "kubelet_image_pull_duration_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletImagePullDurationSeconds"] = m.Value
						break
					}
				}

			// Volume operations
			case "storage_operation_duration_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						operationType := m.Labels["operation_name"]
						volumePlugin := m.Labels["volume_plugin"]
						if operationType != "" {
							metricName := fmt.Sprintf("kubeletStorageOperation_%s_DurationSeconds", operationType)
							nodeMetrics[metricName] = m.Value

							// Also track by volume plugin if available
							if volumePlugin != "" {
								metricName := fmt.Sprintf("kubeletStorageOperation_%s_%s_DurationSeconds", operationType, volumePlugin)
								nodeMetrics[metricName] = m.Value
							}
						}
					}
				}
			case "storage_operation_errors_total":
				for _, m := range family.Metrics {
					operationType := m.Labels["operation_name"]
					if operationType != "" {
						metricName := fmt.Sprintf("kubeletStorageOperation_%s_ErrorsTotal", operationType)
						nodeMetrics[metricName] = m.Value
					}
				}

			// Evictions
			case "kubelet_evictions_total":
				for _, m := range family.Metrics {
					evictionSignal := m.Labels["eviction_signal"]
					if evictionSignal != "" {
						metricName := fmt.Sprintf("kubeletEvictions_%s_Total", evictionSignal)
						nodeMetrics[metricName] = m.Value
					} else {
						nodeMetrics["kubeletEvictionsTotal"] = m.Value
					}
				}
			case "kubelet_eviction_stats_age_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						nodeMetrics["kubeletEvictionStatsAgeSeconds"] = m.Value
						break
					}
				}

			// API client metrics
			case "kubelet_http_requests_total":
				for _, m := range family.Metrics {
					method := m.Labels["method"]
					path := m.Labels["path"]
					if method != "" && path != "" {
						// Simplify path to avoid cardinality explosion
						metricName := fmt.Sprintf("kubeletHTTPRequests_%s_Total", method)
						// Aggregate by method - store directly, will be aggregated by NR backend
						nodeMetrics[metricName] = m.Value
					}
				}
			case "kubelet_http_requests_duration_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						method := m.Labels["method"]
						if method != "" {
							metricName := fmt.Sprintf("kubeletHTTPRequests_%s_DurationSeconds", method)
							nodeMetrics[metricName] = m.Value
						}
					}
				}

			// Node stats
			case "kubelet_running_pods":
				for _, m := range family.Metrics {
					nodeMetrics["kubeletRunningPods"] = m.Value
					break
				}
			case "kubelet_running_containers":
				for _, m := range family.Metrics {
					containerState := m.Labels["container_state"]
					if containerState != "" {
						metricName := fmt.Sprintf("kubeletRunningContainers_%s", containerState)
						nodeMetrics[metricName] = m.Value
					} else {
						nodeMetrics["kubeletRunningContainers"] = m.Value
					}
				}

			// Node name (useful for verification)
			case "kubelet_node_name":
				for _, m := range family.Metrics {
					if nodeLabel, ok := m.Labels["node"]; ok {
						nodeMetrics["kubeletNodeNameMetric"] = nodeLabel
					}
					break
				}

			// Additional important metrics
			case "kubelet_node_config_error":
				for _, m := range family.Metrics {
					nodeMetrics["kubeletNodeConfigError"] = m.Value
					break
				}
			case "kubelet_cgroup_manager_duration_seconds":
				for _, m := range family.Metrics {
					if m.Labels["quantile"] == "0.99" {
						operationType := m.Labels["operation_type"]
						if operationType != "" {
							metricName := fmt.Sprintf("kubeletCgroupManager_%s_DurationSeconds", operationType)
							nodeMetrics[metricName] = m.Value
						}
					}
				}
			}
		}

		// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform)
		// This needs to be a map[string]interface{}, not a JSON string
		if len(nodeMetrics) > 0 {
			metricsDiagnostics := make(map[string]interface{})
			for k, v := range nodeMetrics {
				if len(k) > 7 && k[:7] == "kubelet" {
					metricsDiagnostics[k[7:]] = v
				} else {
					metricsDiagnostics[k] = v
				}
			}
			nodeMetrics["kubeletMetricsDiagnostics"] = metricsDiagnostics
		}

		return g, nil
	}
}

// kubeletHealthQueries defines the Prometheus queries for kubelet health metrics
var kubeletHealthQueries = []prometheus.Query{
	// PLEG metrics - critical for kubelet health
	{MetricName: "kubelet_pleg_relist_duration_seconds"},
	{MetricName: "kubelet_pleg_relist_interval_seconds"},

	// Pod lifecycle metrics
	{MetricName: "kubelet_pod_start_duration_seconds"},
	{MetricName: "kubelet_pod_worker_duration_seconds"},
	{MetricName: "kubelet_pod_worker_start_duration_seconds"},

	// Runtime operations
	{MetricName: "kubelet_runtime_operations_duration_seconds"},
	{MetricName: "kubelet_runtime_operations_errors_total"},
	{MetricName: "kubelet_runtime_operations_total"},

	// Image management
	{MetricName: "kubelet_image_pull_duration_seconds"},

	// Volume operations
	{MetricName: "storage_operation_duration_seconds"},
	{MetricName: "storage_operation_errors_total"},

	// Evictions
	{MetricName: "kubelet_evictions_total"},
	{MetricName: "kubelet_eviction_stats_age_seconds"},

	// API client metrics
	{MetricName: "kubelet_http_requests_total"},
	{MetricName: "kubelet_http_requests_duration_seconds"},

	// Node stats
	{MetricName: "kubelet_running_pods"},
	{MetricName: "kubelet_running_containers"},
	{MetricName: "kubelet_node_name"},

	// Configuration and errors
	{MetricName: "kubelet_node_config_error"},
	{MetricName: "kubelet_cgroup_manager_duration_seconds"},
}
