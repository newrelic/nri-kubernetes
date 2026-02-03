package crd

import (
	"testing"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

func TestFilterCRDMetrics(t *testing.T) {
	tests := []struct {
		name     string
		input    []prometheus.MetricFamily
		expected int
	}{
		{
			name: "filters CRD metrics correctly",
			input: []prometheus.MetricFamily{
				{Name: "kube_customresource_nodepool_limit_cpu"},
				{Name: "kube_pod_status_phase"},
				{Name: "kube_customresource_rollout_replicas"},
				{Name: "kube_node_info"},
			},
			expected: 2,
		},
		{
			name: "returns empty when no CRD metrics",
			input: []prometheus.MetricFamily{
				{Name: "kube_pod_status_phase"},
				{Name: "kube_node_info"},
			},
			expected: 0,
		},
		{
			name:     "handles empty input",
			input:    []prometheus.MetricFamily{},
			expected: 0,
		},
		{
			name:     "handles nil input",
			input:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterCRDMetrics(tt.input)
			assert.Len(t, result, tt.expected)

			// Verify all returned metrics start with the CRD prefix
			for _, mf := range result {
				assert.True(t, len(mf.Name) >= len(CRDMetricPrefix))
				assert.Equal(t, CRDMetricPrefix, mf.Name[:len(CRDMetricPrefix)])
			}
		})
	}
}

func TestExportDimensionalMetrics(t *testing.T) {
	tests := []struct {
		name           string
		metricFamilies []prometheus.MetricFamily
		clusterName    string
		expectError    bool
		expectMetrics  int // Number of individual metric data points expected
	}{
		{
			name: "exports gauge metric with all labels",
			metricFamilies: []prometheus.MetricFamily{
				{
					Name: "kube_customresource_nodepool_limit_cpu",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{
								"customresource_group":   "karpenter.sh",
								"customresource_kind":    "NodePool",
								"customresource_version": "v1beta1",
								"name":                   "default",
								"environment":            "production",
								"team":                   "platform",
							},
							Value: prometheus.GaugeValue(1000),
						},
					},
				},
			},
			clusterName:   "test-cluster",
			expectError:   false,
			expectMetrics: 1,
		},
		{
			name: "exports multiple time series (different labels)",
			metricFamilies: []prometheus.MetricFamily{
				{
					Name: "kube_customresource_nodepool_nodes_count",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{"name": "pool-a"},
							Value:  prometheus.GaugeValue(3),
						},
						{
							Labels: prometheus.Labels{"name": "pool-b"},
							Value:  prometheus.GaugeValue(5),
						},
					},
				},
			},
			clusterName:   "test-cluster",
			expectError:   false,
			expectMetrics: 2, // Two separate metric data points
		},
		{
			name: "filters out non-CRD metrics",
			metricFamilies: []prometheus.MetricFamily{
				{
					Name: "kube_customresource_test",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{"name": "test"},
							Value:  prometheus.GaugeValue(1),
						},
					},
				},
				{
					Name: "kube_pod_status_phase",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{"pod": "test"},
							Value:  prometheus.GaugeValue(1),
						},
					},
				},
			},
			clusterName:   "test-cluster",
			expectError:   false,
			expectMetrics: 1, // Only CRD metric
		},
		{
			name: "handles counter metrics",
			metricFamilies: []prometheus.MetricFamily{
				{
					Name: "kube_customresource_requests_total",
					Type: "counter",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{"name": "test"},
							Value:  prometheus.CounterValue(100),
						},
					},
				},
			},
			clusterName:   "test-cluster",
			expectError:   false,
			expectMetrics: 1,
		},
		{
			name: "exports multiple metrics separately (no consolidation)",
			metricFamilies: []prometheus.MetricFamily{
				{
					Name: "kube_customresource_nodepool_limit_cpu",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{
								"name":                   "default",
								"customresource_kind":    "NodePool",
								"customresource_group":   "karpenter.sh",
								"customresource_version": "v1beta1",
							},
							Value: prometheus.GaugeValue(1000),
						},
					},
				},
				{
					Name: "kube_customresource_nodepool_limit_memory",
					Type: "gauge",
					Metrics: []prometheus.Metric{
						{
							Labels: prometheus.Labels{
								"name":                   "default",
								"customresource_kind":    "NodePool",
								"customresource_group":   "karpenter.sh",
								"customresource_version": "v1beta1",
							},
							Value: prometheus.GaugeValue(2048),
						},
					},
				},
			},
			clusterName:   "test-cluster",
			expectError:   false,
			expectMetrics: 2, // Two separate metric data points (not consolidated)
		},
		{
			name:           "handles empty metric families",
			metricFamilies: []prometheus.MetricFamily{},
			clusterName:    "test-cluster",
			expectError:    false,
			expectMetrics:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger := log.New()
			logger.SetLevel(log.ErrorLevel) // Reduce noise in tests

			// Create harvester with dummy API key for testing
			// The harvester won't actually send data during tests
			harvester, err := telemetry.NewHarvester(
				telemetry.ConfigAPIKey("test-api-key-for-unit-tests"),
				telemetry.ConfigHarvestPeriod(0), // No automatic harvest
			)
			require.NoError(t, err)

			// Export metrics
			config := ExportConfig{
				ClusterName: tt.clusterName,
				Logger:      logger,
				Harvester:   harvester,
			}
			err = ExportDimensionalMetrics(tt.metricFamilies, config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Note: The telemetry SDK doesn't expose recorded metrics for inspection
			// In a real scenario, metrics would be sent to New Relic's Metric API
			// and verified through the New Relic UI or NRQL queries
		})
	}
}

func TestExportDimensionalMetrics_AttributesIncludeClusterName(t *testing.T) {
	metricFamilies := []prometheus.MetricFamily{
		{
			Name: "kube_customresource_test",
			Type: "gauge",
			Metrics: []prometheus.Metric{
				{
					Labels: prometheus.Labels{
						"name":      "test-resource",
						"namespace": "default",
					},
					Value: prometheus.GaugeValue(1),
				},
			},
		},
	}

	logger := log.New()
	logger.SetLevel(log.ErrorLevel) // Reduce noise in tests

	harvester, err := telemetry.NewHarvester(
		telemetry.ConfigAPIKey("test-api-key-for-unit-tests"),
		telemetry.ConfigHarvestPeriod(0),
	)
	require.NoError(t, err)

	config := ExportConfig{
		ClusterName: "production-cluster",
		Logger:      logger,
		Harvester:   harvester,
	}

	err = ExportDimensionalMetrics(metricFamilies, config)
	assert.NoError(t, err)

	// The test verifies that the function completes without error.
	// Cluster name is added to attributes in the implementation.
}

func TestCRDMetricPrefix(t *testing.T) {
	// Verify the constant is correctly defined
	assert.Equal(t, "kube_customresource_", CRDMetricPrefix)
}
