package crd

import (
	"strings"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const (
	// CRDMetricPrefix is the prefix used by KSM for all custom resource metrics.
	CRDMetricPrefix = "kube_customresource_"
)

// IsCRDMetric checks if a metric name is a CRD metric (starts with kube_customresource_).
func IsCRDMetric(metricName string) bool {
	return strings.HasPrefix(metricName, CRDMetricPrefix)
}

// FilterCRDMetrics filters metric families to only include CRD metrics (those starting with kube_customresource_).
func FilterCRDMetrics(metricFamilies []prometheus.MetricFamily) []prometheus.MetricFamily {
	var crdMetrics []prometheus.MetricFamily

	for _, mf := range metricFamilies {
		if strings.HasPrefix(mf.Name, CRDMetricPrefix) {
			crdMetrics = append(crdMetrics, mf)
		}
	}

	return crdMetrics
}

// ExportConfig holds configuration for CRD metric export.
type ExportConfig struct {
	ClusterName string
	Logger      *log.Logger
	Harvester   *telemetry.Harvester
}

// extractMetricValue extracts the numeric value from a Prometheus metric.
// Returns the value and a boolean indicating success.
func extractMetricValue(promMetric prometheus.Metric) (float64, bool) {
	switch v := promMetric.Value.(type) {
	case prometheus.GaugeValue:
		return float64(v), true
	case prometheus.CounterValue:
		return float64(v), true
	case prometheus.UntypedValue:
		return float64(v), true
	default:
		return 0, false
	}
}

// buildMetricAttributes creates attributes map from Prometheus labels and cluster name.
func buildMetricAttributes(clusterName string, labels map[string]string) map[string]interface{} {
	attributes := make(map[string]interface{})
	attributes["clusterName"] = clusterName

	for labelName, labelValue := range labels {
		attributes[labelName] = labelValue
	}

	return attributes
}

// recordMetrics sends metrics to the harvester and logs debug information.
func recordMetrics(metrics []telemetry.Metric, metricsCount int, crdMetricsCount int, logger *log.Logger, harvester *telemetry.Harvester) {
	if len(metrics) == 0 {
		return
	}

	logger.Infof("Recording %d dimensional metrics to harvester", metricsCount)
	for i, m := range metrics {
		harvester.RecordMetric(m)
		// Log first few metrics for debugging
		if i < 5 { //nolint:mnd // Debug logging limit
			if g, ok := m.(telemetry.Gauge); ok {
				logger.Debugf("  Metric: %s = %f (attributes: %d)", g.Name, g.Value, len(g.Attributes))
			}
		}
	}
	logger.Infof("Successfully recorded %d dimensional metrics from %d metric families", metricsCount, crdMetricsCount)
}

// ExportDimensionalMetrics exports CRD metrics as dimensional metrics to the Metric table.
// Each Prometheus time series becomes a separate metric data point with all labels as attributes.
//
// Example query: FROM Metric SELECT * WHERE metricName = 'kube_customresource_nodepool_limit_cpu'.
func ExportDimensionalMetrics(metricFamilies []prometheus.MetricFamily, config ExportConfig) error {
	crdMetrics := FilterCRDMetrics(metricFamilies)

	if len(crdMetrics) == 0 {
		config.Logger.Debug("No CRD metrics found to export")
		return nil
	}

	config.Logger.Debugf("Exporting %d CRD metric families as dimensional metrics", len(crdMetrics))

	var metrics []telemetry.Metric
	metricsCount := 0

	// Convert each Prometheus metric time series to a New Relic dimensional metric
	for _, metricFamily := range crdMetrics {
		metricName := metricFamily.Name
		config.Logger.Tracef("Processing CRD metric family: %s (type: %s)", metricName, metricFamily.Type)

		// Each metric family can have multiple time series (different label combinations)
		for _, promMetric := range metricFamily.Metrics {
			// Extract value - supports GAUGE, COUNTER, and UNTYPED (includes OpenMetrics StateSet and Info types)
			value, ok := extractMetricValue(promMetric)
			if !ok {
				config.Logger.Tracef("Skipping metric %s (type=%s): value type %T not supported",
					metricName, metricFamily.Type, promMetric.Value)
				continue
			}

			// Build attributes from all Prometheus labels plus clusterName
			attributes := buildMetricAttributes(config.ClusterName, promMetric.Labels)

			// Create gauge metric (both Prometheus gauges and counters become New Relic gauges)
			metric := telemetry.Gauge{
				Name:       metricName,
				Value:      value,
				Attributes: attributes,
			}

			metrics = append(metrics, metric)
			metricsCount++
		}
	}

	// Send metrics via harvester
	recordMetrics(metrics, metricsCount, len(crdMetrics), config.Logger, config.Harvester)

	return nil
}
