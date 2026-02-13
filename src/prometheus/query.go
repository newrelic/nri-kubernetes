package prometheus

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	model "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	prometheusmodel "github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
)

// TODO: See https://github.com/prometheus/prom2json/blob/master/prom2json.go#L171 for how to connect, how to parse plain text, etc

// QueryOperator indicates the operator used for the query.
type QueryOperator int

const (
	// QueryOpAnd Is the default operator. Means all values should match.
	QueryOpAnd QueryOperator = iota

	// QueryOpNor means all values should not match.
	QueryOpNor
)

// Query represents the query object. It will run against Prometheus metrics.
type Query struct {
	CustomName string
	MetricName string
	Labels     QueryLabels
	Value      QueryValue // TODO Only supported Counter and Gauge
}

// QueryValue represents the query for a value.
type QueryValue struct {
	Operator QueryOperator
	Value    Value
}

// QueryLabels represents the query for labels.
type QueryLabels struct {
	Operator QueryOperator
	Labels   Labels
}

// Execute runs the query.
func (q Query) Execute(promMetricFamily *model.MetricFamily) (metricFamily MetricFamily) {
	if promMetricFamily.GetName() != q.MetricName {
		return
	}

	if len(promMetricFamily.Metric) <= 0 {
		// Should not happen
		return
	}
	var matches []Metric
	for _, promMetric := range promMetricFamily.Metric {
		if len(q.Labels.Labels) > 0 {
			// Match by labels
			switch q.Labels.Operator {
			case QueryOpAnd:
				if !q.Labels.Labels.AreIn(promMetric.Label) {
					continue
				}
			case QueryOpNor:
				if q.Labels.Labels.AreIn(promMetric.Label) {
					continue
				}
			}
		}

		value := valueFromPrometheus(promMetricFamily.GetType(), promMetric)

		if q.Value.Value != nil {
			switch q.Value.Operator {
			case QueryOpAnd:
				if q.Value.Value.String() != value.String() {
					continue
				}
			case QueryOpNor:
				if q.Value.Value.String() == value.String() {
					continue
				}
			}
		}

		m := Metric{
			Labels: labelsFromPrometheus(promMetric.Label),
			Value:  value,
		}

		matches = append(matches, m)
	}

	var name string
	if q.CustomName != "" {
		name = q.CustomName
	} else {
		name = promMetricFamily.GetName()
	}

	metricFamily = MetricFamily{
		Name:    name,
		Type:    promMetricFamily.GetType().String(),
		Metrics: matches,
	}

	return
}

func valueFromPrometheus(metricType model.MetricType, metric *model.Metric) Value {
	switch metricType {
	case model.MetricType_COUNTER:
		return CounterValue(metric.Counter.GetValue())
	case model.MetricType_GAUGE:
		return GaugeValue(metric.Gauge.GetValue())
	case model.MetricType_HISTOGRAM:
		// Not supported yet
		fallthrough
	case model.MetricType_SUMMARY:
		return metric.Summary
	case model.MetricType_UNTYPED:
		// Not supported yet
		fallthrough
	default:
		return EmptyValue
	}
}

const (
	// Minimum number of fields in a TYPE declaration: "# TYPE metric_name metric_type".
	minTypeFields = 4
	// Minimum number of fields in a HELP declaration: "# HELP metric_name description".
	minHelpFields = 3
)

// isUnsupportedMetricType checks if a metric type is unsupported by prometheus/client_model.
// OpenMetrics 1.0 types "info" and "stateset" are not supported.
func isUnsupportedMetricType(metricType string) bool {
	return metricType == "info" || metricType == "stateset"
}

// handleTypeDeclaration processes a TYPE declaration line and determines if the metric family
// should be skipped. Returns (shouldSkip, metricName).
func handleTypeDeclaration(line string, logger *log.Logger) (bool, string) {
	parts := strings.Fields(line)
	if len(parts) < minTypeFields {
		return false, ""
	}

	metricName := parts[2]
	metricType := parts[3]

	if isUnsupportedMetricType(metricType) {
		logger.Debugf("Skipping unsupported metric type '%s' for metric '%s'", metricType, metricName)
		return true, metricName
	}

	return false, ""
}

// handleHelpDeclaration processes a HELP declaration line and updates skip state if we
// encounter a different metric family. Returns (shouldSkip, currentMetricName).
func handleHelpDeclaration(line string, currentlySkipping bool, currentMetricName string) (bool, string) {
	parts := strings.Fields(line)
	if len(parts) < minHelpFields {
		return currentlySkipping, currentMetricName
	}

	metricName := parts[2]
	// If we were skipping and now see a different metric, stop skipping
	if currentlySkipping && currentMetricName != "" && metricName != currentMetricName {
		return false, ""
	}

	return currentlySkipping, currentMetricName
}

// shouldSkipLine determines if a line should be skipped when filtering an unsupported metric family.
func shouldSkipLine(line string, currentMetricName string) bool {
	// Skip HELP line for the current unsupported metric
	if strings.HasPrefix(line, "# HELP ") {
		parts := strings.Fields(line)
		if len(parts) >= minHelpFields && parts[2] == currentMetricName {
			return true
		}
	}
	// Skip metric data lines (non-comment lines)
	return !strings.HasPrefix(line, "#")
}

// filterUnsupportedMetrics preprocesses Prometheus exposition format text to remove
// metric families with unsupported types (e.g., OpenMetrics "info", "stateset").
//
// The Prometheus TextParser (expfmt.TextParser) fails when it encounters unknown metric types,
// stopping parsing and losing all subsequent metrics. This function removes those problematic
// metric families before parsing.
//
// Returns a new io.Reader with filtered content and a list of skipped metric names.
func filterUnsupportedMetrics(body io.Reader, logger *log.Logger) (io.Reader, []string, error) {
	scanner := bufio.NewScanner(body)

	const maxScanTokenSize = 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	var filteredLines []string
	var skippedMetrics []string
	var skipUntilNextFamily bool
	var currentMetricName string

	for scanner.Scan() {
		line := scanner.Text() // Preserve original formatting

		trimmedLine := strings.TrimSpace(line)

		// Preserve empty lines
		if trimmedLine == "" {
			if !skipUntilNextFamily {
				filteredLines = append(filteredLines, line)
			}
			continue
		}

		// Check if this is a TYPE declaration
		if strings.HasPrefix(trimmedLine, "# TYPE ") {
			skipUntilNextFamily, currentMetricName = handleTypeDeclaration(trimmedLine, logger)
			if skipUntilNextFamily {
				skippedMetrics = append(skippedMetrics, currentMetricName)
				continue
			}
		}

		// Check if this is a HELP declaration for a new metric family
		if strings.HasPrefix(trimmedLine, "# HELP ") {
			skipUntilNextFamily, currentMetricName = handleHelpDeclaration(
				trimmedLine, skipUntilNextFamily, currentMetricName,
			)
		}

		// Skip metric data lines if we're in an unsupported family
		if skipUntilNextFamily && shouldSkipLine(trimmedLine, currentMetricName) {
			continue
		}

		// Keep this line
		filteredLines = append(filteredLines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, skippedMetrics, fmt.Errorf("reading metrics body: %w", err)
	}

	// Reconstruct the filtered text
	filtered := strings.Join(filteredLines, "\n") + "\n" // Add trailing newline
	return bytes.NewBufferString(filtered), skippedMetrics, nil
}

/**
 * Try our best to parse a response. Even if an error is encountered
 * midway through parsing we will put into the receiving channel any
 * metric families found along the way. We also return any error that
 * we did come along. Fail-fast, best attempt behavior.
 */
func parseResponse(resp *http.Response, ch chan<- *model.MetricFamily, logger *log.Logger) error {
	defer close(ch)

	// Filter out unsupported metric types before parsing to prevent parser from failing.
	// This solves issue #1293 where OpenMetrics "info" types cause complete data loss.
	filtered, skippedMetrics, err := filterUnsupportedMetrics(resp.Body, logger)
	if err != nil {
		return fmt.Errorf("filtering unsupported metrics: %w", err)
	}

	if len(skippedMetrics) > 0 {
		logger.Infof("Skipped %d metric families with unsupported OpenMetrics types: %v", len(skippedMetrics), skippedMetrics)
	}

	// Use NewTextParser with UTF8 validation scheme instead of zero-value TextParser.
	// The zero-value TextParser has an unset validation scheme which causes a panic.
	parser := expfmt.NewTextParser(prometheusmodel.UTF8Validation)
	metricFamilies, err := parser.TextToMetricFamilies(filtered)
	if err != nil {
		err = fmt.Errorf("reading text format failed: %w", err)
	}

	for _, mf := range metricFamilies {
		ch <- mf
	}

	return err
}

func handleResponseWithFilter(resp *http.Response, queries []Query, logger *log.Logger) ([]MetricFamily, error) {
	if resp == nil {
		return nil, fmt.Errorf("response cannot be nil")
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error calling prometheus exposed metrics endpoint. Got status code: %d", resp.StatusCode)
	}

	metrics := make([]MetricFamily, 0)
	ch := make(chan *model.MetricFamily)

	var err error
	go func() {
		err = parseResponse(resp, ch, logger)
	}()

	for promMetricFamily := range ch {
		for _, q := range queries {
			f := q.Execute(promMetricFamily)
			if f.valid() {
				metrics = append(metrics, f)
			}
		}
	}

	// parseResponse does some lenient parsing so metrics may be non-empty
	// even when err is non-nil. We handle the cases here
	if err != nil && len(metrics) > 0 {
		// be lenient: log error case but don't bubble up failure
		logger.Errorf("Failed while trying to parse metrics: %v", err)
		err = nil
	}

	if err != nil {
		return nil, fmt.Errorf("parsing metrics: %w", err)
	}

	return metrics, nil
}

// MetricFamiliesGetFunc is the interface satisfied by prometheus Client.
// TODO: This whole flow is too convoluted, we should refactor and rename this.
type MetricFamiliesGetFunc interface {
	// MetricFamiliesGetFunc returns a prometheus.FilteredFetcher configured to get KSM metrics from and endpoint.
	// prometheus.FilteredFetcher will be used by the prometheus client to scrape and filter metrics.
	MetricFamiliesGetFunc(url string) FetchAndFilterMetricsFamilies
}

type FetchAndFilterMetricsFamilies func([]Query) ([]MetricFamily, error)

func GetFilteredMetricFamilies(httpClient client.HTTPDoer, url string, queries []Query, logger *log.Logger) ([]MetricFamily, error) {
	logger.Debugf("Calling a prometheus endpoint: %s", url)

	// todo it would be nice to have context with deadline
	req, err := NewRequest(url)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metrics from %q: %w", url, err)
	}

	return handleResponseWithFilter(resp, queries, logger)
}

func labelsFromPrometheus(pairs []*model.LabelPair) Labels {
	labels := make(Labels)
	for _, p := range pairs {
		labels[p.GetName()] = p.GetValue()
	}

	return labels
}
