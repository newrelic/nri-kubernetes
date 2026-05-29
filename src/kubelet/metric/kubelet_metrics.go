package metric

import (
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const (
	// KubeletMetricsPath is the path where kubelet serves its own health/performance metrics.
	KubeletMetricsPath = "/metrics"
)

// metricsProcessor handles the processing of kubelet metrics.
type metricsProcessor struct {
	nodeMetrics definition.RawMetrics
}

// KubeletMetricsFetchFunc creates a FetchFunc that fetches kubelet's own health metrics.
func KubeletMetricsFetchFunc(fetchAndFilterPrometheus prometheus.FetchAndFilterMetricsFamilies, nodeName string) data.FetchFunc {
	return func() (definition.RawGroups, error) {
		families, err := fetchAndFilterPrometheus(GetKubeletHealthQueries())
		if err != nil {
			return nil, fmt.Errorf("error requesting kubelet metrics endpoint: %w", err)
		}

		g := definition.RawGroups{
			"node": {
				nodeName: make(definition.RawMetrics),
			},
		}

		processor := &metricsProcessor{nodeMetrics: g["node"][nodeName]}
		processor.processAllFamilies(families)
		processor.addDiagnosticsMap()

		return g, nil
	}
}

func (p *metricsProcessor) processAllFamilies(families []prometheus.MetricFamily) {
	for _, family := range families {
		p.processFamilyByName(family)
	}
}

//nolint:gocyclo,cyclop // Switching on metric names is inherently complex but necessary.
func (p *metricsProcessor) processFamilyByName(family prometheus.MetricFamily) {
	switch family.Name {
	case "kubelet_pleg_relist_duration_seconds":
		p.processQuantileMetric(family, "kubeletPLEGRelistDurationSeconds")
	case "kubelet_pleg_relist_interval_seconds":
		p.processQuantileMetric(family, "kubeletPLEGRelistIntervalSeconds")
	case "kubelet_pod_start_duration_seconds":
		p.processQuantileMetric(family, "kubeletPodStartDurationSeconds")
	case "kubelet_pod_worker_duration_seconds":
		p.processQuantileMetric(family, "kubeletPodWorkerDurationSeconds")
	case "kubelet_runtime_operations_duration_seconds":
		p.processRuntimeOperationsDuration(family)
	case "kubelet_runtime_operations_errors_total":
		p.processRuntimeOperationsErrors(family)
	case "kubelet_runtime_operations_total":
		p.processRuntimeOperationsTotal(family)
	case "kubelet_image_pull_duration_seconds":
		p.processQuantileMetric(family, "kubeletImagePullDurationSeconds")
	case "storage_operation_duration_seconds":
		p.processStorageOperationsDuration(family)
	case "storage_operation_errors_total":
		p.processStorageOperationsErrors(family)
	case "kubelet_evictions_total":
		p.processEvictions(family)
	case "kubelet_eviction_stats_age_seconds":
		p.processQuantileMetric(family, "kubeletEvictionStatsAgeSeconds")
	case "kubelet_http_requests_total":
		p.processHTTPRequestsTotal(family)
	case "kubelet_http_requests_duration_seconds":
		p.processHTTPRequestsDuration(family)
	case "kubelet_running_pods":
		p.processSimpleMetric(family, "kubeletRunningPods")
	case "kubelet_running_containers":
		p.processRunningContainers(family)
	case "kubelet_node_name":
		p.processNodeName(family)
	case "kubelet_node_config_error":
		p.processSimpleMetric(family, "kubeletNodeConfigError")
	case "kubelet_cgroup_manager_duration_seconds":
		p.processCgroupManagerDuration(family)
	}
}

func (p *metricsProcessor) processQuantileMetric(family prometheus.MetricFamily, metricName string) {
	for _, m := range family.Metrics {
		if m.Labels["quantile"] == quantile99 {
			p.nodeMetrics[metricName] = m.Value
			return
		}
	}
}

func (p *metricsProcessor) processSimpleMetric(family prometheus.MetricFamily, metricName string) {
	for _, m := range family.Metrics {
		p.nodeMetrics[metricName] = m.Value
		return
	}
}

func (p *metricsProcessor) processRuntimeOperationsDuration(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		if m.Labels["quantile"] == quantile99 {
			operationType := m.Labels["operation_type"]
			if operationType != "" {
				metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_DurationSeconds", operationType)
				p.nodeMetrics[metricName] = m.Value
			}
		}
	}
}

func (p *metricsProcessor) processRuntimeOperationsErrors(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		operationType := m.Labels["operation_type"]
		if operationType != "" {
			metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_ErrorsTotal", operationType)
			p.nodeMetrics[metricName] = m.Value
		}
	}
}

func (p *metricsProcessor) processRuntimeOperationsTotal(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		operationType := m.Labels["operation_type"]
		if operationType != "" {
			metricName := fmt.Sprintf("kubeletRuntimeOperation_%s_Total", operationType)
			p.nodeMetrics[metricName] = m.Value
		}
	}
}

func (p *metricsProcessor) processStorageOperationsDuration(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		if m.Labels["quantile"] == quantile99 {
			operationType := m.Labels["operation_name"]
			volumePlugin := m.Labels["volume_plugin"]
			if operationType != "" {
				metricName := fmt.Sprintf("kubeletStorageOperation_%s_DurationSeconds", operationType)
				p.nodeMetrics[metricName] = m.Value

				// Also track by volume plugin if available.
				if volumePlugin != "" {
					metricNameWithPlugin := fmt.Sprintf("kubeletStorageOperation_%s_%s_DurationSeconds", operationType, volumePlugin)
					p.nodeMetrics[metricNameWithPlugin] = m.Value
				}
			}
		}
	}
}

func (p *metricsProcessor) processStorageOperationsErrors(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		operationType := m.Labels["operation_name"]
		if operationType != "" {
			metricName := fmt.Sprintf("kubeletStorageOperation_%s_ErrorsTotal", operationType)
			p.nodeMetrics[metricName] = m.Value
		}
	}
}

func (p *metricsProcessor) processEvictions(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		evictionSignal := m.Labels["eviction_signal"]
		if evictionSignal != "" {
			metricName := fmt.Sprintf("kubeletEvictions_%s_Total", evictionSignal)
			p.nodeMetrics[metricName] = m.Value
		} else {
			p.nodeMetrics["kubeletEvictionsTotal"] = m.Value
		}
	}
}

func (p *metricsProcessor) processHTTPRequestsTotal(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		method := m.Labels["method"]
		path := m.Labels["path"]
		if method != "" && path != "" {
			// Simplify path to avoid cardinality explosion.
			metricName := fmt.Sprintf("kubeletHTTPRequests_%s_Total", method)
			// Aggregate by method - store directly, will be aggregated by NR backend.
			p.nodeMetrics[metricName] = m.Value
		}
	}
}

func (p *metricsProcessor) processHTTPRequestsDuration(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		if m.Labels["quantile"] == quantile99 {
			method := m.Labels["method"]
			if method != "" {
				metricName := fmt.Sprintf("kubeletHTTPRequests_%s_DurationSeconds", method)
				p.nodeMetrics[metricName] = m.Value
			}
		}
	}
}

func (p *metricsProcessor) processRunningContainers(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		containerState := m.Labels["container_state"]
		if containerState != "" {
			metricName := fmt.Sprintf("kubeletRunningContainers_%s", containerState)
			p.nodeMetrics[metricName] = m.Value
		} else {
			p.nodeMetrics["kubeletRunningContainers"] = m.Value
		}
	}
}

func (p *metricsProcessor) processNodeName(family prometheus.MetricFamily) {
	if len(family.Metrics) == 0 {
		return
	}
	// Only need the first metric for node name.
	if nodeLabel, ok := family.Metrics[0].Labels["node"]; ok {
		p.nodeMetrics["kubeletNodeNameMetric"] = nodeLabel
	}
}

func (p *metricsProcessor) processCgroupManagerDuration(family prometheus.MetricFamily) {
	for _, m := range family.Metrics {
		if m.Labels["quantile"] == quantile99 {
			operationType := m.Labels["operation_type"]
			if operationType != "" {
				metricName := fmt.Sprintf("kubeletCgroupManager_%s_DurationSeconds", operationType)
				p.nodeMetrics[metricName] = m.Value
			}
		}
	}
}

func (p *metricsProcessor) addDiagnosticsMap() {
	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform).
	// This needs to be a map[string]interface{}, not a JSON string.
	if len(p.nodeMetrics) > 0 {
		metricsDiagnostics := make(map[string]interface{})
		for k, v := range p.nodeMetrics {
			if len(k) > 7 && k[:7] == kubeletPrefix {
				metricsDiagnostics[k[7:]] = v
			} else {
				metricsDiagnostics[k] = v
			}
		}
		p.nodeMetrics["kubeletMetricsDiagnostics"] = metricsDiagnostics
	}
}

// GetKubeletHealthQueries returns the Prometheus queries for kubelet health metrics.
func GetKubeletHealthQueries() []prometheus.Query {
	return []prometheus.Query{
		// PLEG metrics - critical for kubelet health.
		{MetricName: "kubelet_pleg_relist_duration_seconds"},
		{MetricName: "kubelet_pleg_relist_interval_seconds"},

		// Pod lifecycle metrics.
		{MetricName: "kubelet_pod_start_duration_seconds"},
		{MetricName: "kubelet_pod_worker_duration_seconds"},
		{MetricName: "kubelet_pod_worker_start_duration_seconds"},

		// Runtime operations.
		{MetricName: "kubelet_runtime_operations_duration_seconds"},
		{MetricName: "kubelet_runtime_operations_errors_total"},
		{MetricName: "kubelet_runtime_operations_total"},

		// Image management.
		{MetricName: "kubelet_image_pull_duration_seconds"},

		// Volume operations.
		{MetricName: "storage_operation_duration_seconds"},
		{MetricName: "storage_operation_errors_total"},

		// Evictions.
		{MetricName: "kubelet_evictions_total"},
		{MetricName: "kubelet_eviction_stats_age_seconds"},

		// API client metrics.
		{MetricName: "kubelet_http_requests_total"},
		{MetricName: "kubelet_http_requests_duration_seconds"},

		// Node stats.
		{MetricName: "kubelet_running_pods"},
		{MetricName: "kubelet_running_containers"},
		{MetricName: "kubelet_node_name"},

		// Configuration and errors.
		{MetricName: "kubelet_node_config_error"},
		{MetricName: "kubelet_cgroup_manager_duration_seconds"},
	}
}
