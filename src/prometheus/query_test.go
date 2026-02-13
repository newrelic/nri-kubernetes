package prometheus

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	model "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestLabelsAreIn(t *testing.T) {
	expectedLabels := Labels{
		"namespace": "default",
		"pod":       "nr-123456789",
	}

	l := []*model.LabelPair{
		{
			Name:  proto.String("condition"),
			Value: proto.String("false"),
		},
		{
			Name:  proto.String("namespace"),
			Value: proto.String("default"),
		},
		{
			Name:  proto.String("pod"),
			Value: proto.String("nr-123456789"),
		},
	}

	assert.True(t, expectedLabels.AreIn(l))
}

func TestQueryMatch(t *testing.T) {
	queryAnd := Query{
		MetricName: "kube_pod_status_phase",
		Labels: QueryLabels{
			Labels: Labels{
				"namespace": "default",
				"pod":       "nr-123456789",
			},
		},
		Value: QueryValue{
			Value: GaugeValue(1),
		},
	}

	queryNor := Query{
		MetricName: queryAnd.MetricName,
		Labels:     queryAnd.Labels,
		Value: QueryValue{
			Operator: QueryOpNor,
			Value:    GaugeValue(1),
		},
	}

	metrictType := model.MetricType_GAUGE
	r := model.MetricFamily{
		Name: proto.String(queryAnd.MetricName),
		Type: &metrictType,
		Metric: []*model.Metric{
			{
				Gauge: &model.Gauge{
					Value: proto.Float64(1),
				},
				Label: []*model.LabelPair{
					{
						Name:  proto.String("namespace"),
						Value: proto.String("default"),
					},
					{
						Name:  proto.String("pod"),
						Value: proto.String("nr-123456789"),
					},
				},
			},
			{
				Gauge: &model.Gauge{
					Value: proto.Float64(0),
				},
				Label: []*model.LabelPair{
					{
						Name:  proto.String("namespace"),
						Value: proto.String("default"),
					},
					{
						Name:  proto.String("pod"),
						Value: proto.String("nr-123456789"),
					},
				},
			},
		},
	}

	expectedAndOperatorMetrics := MetricFamily{
		Name: queryAnd.MetricName,
		Type: "GAUGE",
		Metrics: []Metric{
			{
				Labels: queryAnd.Labels.Labels,
				Value:  GaugeValue(1),
			},
		},
	}

	expectedNorOperatorMetrics := MetricFamily{
		Name: queryNor.MetricName,
		Type: "GAUGE",
		Metrics: []Metric{
			{
				Labels: queryNor.Labels.Labels,
				Value:  GaugeValue(0),
			},
		},
	}

	assert.Equal(t, expectedAndOperatorMetrics, queryAnd.Execute(&r))
	assert.Equal(t, expectedNorOperatorMetrics, queryNor.Execute(&r))
}

func TestQueryMatch_CustomName(t *testing.T) {
	q := Query{
		CustomName: "custom_name",
		MetricName: "kube_pod_status_phase",
		Labels: QueryLabels{
			Labels: Labels{
				"namespace": "default",
				"pod":       "nr-123456789",
			},
		},
		Value: QueryValue{
			Value: GaugeValue(1),
		},
	}

	metrictType := model.MetricType_GAUGE
	r := model.MetricFamily{
		Name: proto.String(q.MetricName),
		Type: &metrictType,
		Metric: []*model.Metric{
			{
				Gauge: &model.Gauge{
					Value: proto.Float64(1),
				},
				Label: []*model.LabelPair{
					{
						Name:  proto.String("namespace"),
						Value: proto.String("default"),
					},
					{
						Name:  proto.String("pod"),
						Value: proto.String("nr-123456789"),
					},
				},
			},
			{
				Gauge: &model.Gauge{
					Value: proto.Float64(0),
				},
				Label: []*model.LabelPair{
					{
						Name:  proto.String("namespace"),
						Value: proto.String("default"),
					},
					{
						Name:  proto.String("pod"),
						Value: proto.String("nr-123456789"),
					},
				},
			},
		},
	}

	expectedMetrics := MetricFamily{
		Name: q.CustomName,
		Type: "GAUGE",
		Metrics: []Metric{
			{
				Labels: q.Labels.Labels,
				Value:  GaugeValue(1),
			},
		},
	}

	assert.Equal(t, expectedMetrics, q.Execute(&r))
}

//nolint:bodyclose
func TestParseResponse(t *testing.T) {
	t.Parallel()

	chOne := make(chan *model.MetricFamily)
	chTwo := make(chan *model.MetricFamily)

	handlerOne := func(w http.ResponseWriter) {
		_, err := io.WriteString(w,
			`# HELP kube_pod_status_phase The pods current phase. 
			 # TYPE kube_pod_status_phase gauge
			 kube_pod_status_phase{namespace="default",pod="123456789"} 1
			 # HELP kube_custom_elasticsearch_health_status Elasticsearch CRD health status
			 # TYPE kube_custom_elasticsearch_health_status stateset
			 kube_custom_elasticsearch_health_status {customresource_group="elasticsearch.k8s.elastic.co"} 1
			`)
		assert.Nil(t, err)
	}
	handlerTwo := func(w http.ResponseWriter) {
		_, err := io.WriteString(w,
			`# HELP kube_custom_elasticsearch_health_status Elasticsearch CRD health status
			 # TYPE kube_custom_elasticsearch_health_status stateset
			 kube_custom_elasticsearch_health_status {customresource_group="elasticsearch.k8s.elastic.co"} 1
			 # HELP kube_pod_status_phase The pods current phase.
			 # TYPE kube_pod_status_phase gauge
			 kube_pod_status_phase{namespace="default",pod="123456789"} 1
			`)
		assert.Nil(t, err)
	}

	wOne := httptest.NewRecorder()
	wTwo := httptest.NewRecorder()

	handlerOne(wOne)
	handlerTwo(wTwo)
	responseOne := wOne.Result()
	responseTwo := wTwo.Result()

	defer responseOne.Body.Close()
	defer responseTwo.Body.Close()

	logger := logutil.Discard

	var errOne error
	var errTwo error
	go func() {
		errOne = parseResponse(responseOne, chOne, logger)
	}()
	go func() {
		errTwo = parseResponse(responseTwo, chTwo, logger)
	}()

	var oneFamilies int
	var twoFamilies int
	for mf := range chOne {
		_ = mf
		oneFamilies++
	}
	for mf := range chTwo {
		_ = mf
		twoFamilies++
	}

	// With the new filtering logic, unsupported metric types are filtered out,
	// so parsing should succeed and we should get all supported metrics regardless of position.
	assert.Equal(t, 1, oneFamilies, "Should parse gauge metric before stateset")
	assert.Equal(t, 1, twoFamilies, "Should filter out stateset and parse gauge metric that comes after")
	assert.Nil(t, errOne, "Should not error when stateset comes after supported types")
	assert.Nil(t, errTwo, "Should not error when stateset is filtered out")
}

// verifyReplicaSetMetrics is a helper function that verifies metric families contain
// the expected ReplicaSet metrics with correct names, values, and no info metrics.
func verifyReplicaSetMetrics(t *testing.T, metricFamilies []*model.MetricFamily, expectedMetricNames map[string]bool) {
	t.Helper()

	for _, mf := range metricFamilies {
		assert.True(t, expectedMetricNames[mf.GetName()], "Unexpected metric family: %s", mf.GetName())
		assert.NotEqual(t, "kube_gitrepository_resource_info", mf.GetName(), "Info metric should have been filtered out")

		// Verify metrics have the expected labels and values
		if mf.GetName() == "kube_replicaset_created" {
			assert.Len(t, mf.GetMetric(), 1, "Should have 1 metric")
			assert.Equal(t, float64(1620000000), mf.GetMetric()[0].GetGauge().GetValue())
		} else if mf.GetName() == "kube_replicaset_status_replicas" {
			assert.Len(t, mf.GetMetric(), 1, "Should have 1 metric")
			assert.Equal(t, float64(3), mf.GetMetric()[0].GetGauge().GetValue())
		}
	}
}

// TestParseResponseWithInfoMetric tests that "info" type metrics (OpenMetrics 1.0)
// are filtered out gracefully without losing subsequent metrics.
// This test reproduces and validates the fix for issue #1293 where FluxCD info metrics
// appear before ReplicaSet metrics, causing complete data loss.
func TestParseResponseWithInfoMetric(t *testing.T) {
	t.Parallel()

	chOne := make(chan *model.MetricFamily)
	chTwo := make(chan *model.MetricFamily)

	// Scenario 1: info metric BEFORE replicaset metrics (reproduces issue #1293)
	handlerInfoFirst := func(w http.ResponseWriter) {
		_, err := io.WriteString(w,
			`# HELP kube_gitrepository_resource_info The current state of a GitOps Toolkit resource
			 # TYPE kube_gitrepository_resource_info info
			 kube_gitrepository_resource_info{name="podinfo",exported_namespace="flux-system",ready="True",suspended="false"} 1
			 # HELP kube_replicaset_created ReplicaSet creation timestamp
			 # TYPE kube_replicaset_created gauge
			 kube_replicaset_created{namespace="default",replicaset="nginx-123"} 1620000000
			 # HELP kube_replicaset_status_replicas Number of replicas
			 # TYPE kube_replicaset_status_replicas gauge
			 kube_replicaset_status_replicas{namespace="default",replicaset="nginx-123"} 3
			`)
		assert.Nil(t, err)
	}

	// Scenario 2: info metric AFTER replicaset metrics
	handlerInfoLast := func(w http.ResponseWriter) {
		_, err := io.WriteString(w,
			`# HELP kube_replicaset_created ReplicaSet creation timestamp
			 # TYPE kube_replicaset_created gauge
			 kube_replicaset_created{namespace="default",replicaset="nginx-123"} 1620000000
			 # HELP kube_replicaset_status_replicas Number of replicas
			 # TYPE kube_replicaset_status_replicas gauge
			 kube_replicaset_status_replicas{namespace="default",replicaset="nginx-123"} 3
			 # HELP kube_gitrepository_resource_info The current state of a GitOps Toolkit resource
			 # TYPE kube_gitrepository_resource_info info
			 kube_gitrepository_resource_info{name="podinfo",exported_namespace="flux-system",ready="True",suspended="false"} 1
			`)
		assert.Nil(t, err)
	}

	wOne := httptest.NewRecorder()
	wTwo := httptest.NewRecorder()

	handlerInfoFirst(wOne)
	handlerInfoLast(wTwo)
	responseOne := wOne.Result()
	responseTwo := wTwo.Result()

	defer responseOne.Body.Close()
	defer responseTwo.Body.Close()

	logger := logutil.Discard

	var errOne error
	var errTwo error
	go func() {
		errOne = parseResponse(responseOne, chOne, logger)
	}()
	go func() {
		errTwo = parseResponse(responseTwo, chTwo, logger)
	}()

	// Pre-allocate slices with expected capacity
	metricFamiliesOne := make([]*model.MetricFamily, 0, 2)
	metricFamiliesTwo := make([]*model.MetricFamily, 0, 2)

	for mf := range chOne {
		metricFamiliesOne = append(metricFamiliesOne, mf)
	}
	for mf := range chTwo {
		metricFamiliesTwo = append(metricFamiliesTwo, mf)
	}

	// Both scenarios should succeed and return ReplicaSet metrics.
	// The info metrics should be filtered out transparently before parsing.
	// This validates the fix for issue #1293.
	assert.Nil(t, errOne, "Should not error when info type is filtered out")
	assert.Nil(t, errTwo, "Should not error when info type is filtered out")

	// Verify we got exactly 2 metric families (ReplicaSet metrics only, info filtered out)
	assert.Len(t, metricFamiliesOne, 2, "Should parse both ReplicaSet metrics even when info comes first (issue #1293)")
	assert.Len(t, metricFamiliesTwo, 2, "Should parse both ReplicaSet metrics when info comes last")

	// Verify the metric families are the expected ReplicaSet metrics
	expectedMetricNames := map[string]bool{
		"kube_replicaset_created":         true,
		"kube_replicaset_status_replicas": true,
	}

	verifyReplicaSetMetrics(t, metricFamiliesOne, expectedMetricNames)
	verifyReplicaSetMetrics(t, metricFamiliesTwo, expectedMetricNames)
}

// TestFilterUnsupportedMetrics_LargeLines tests that the filter can handle metric lines
// exceeding the default bufio.Scanner buffer size (64KB).
func TestFilterUnsupportedMetrics_LargeLines(t *testing.T) {
	t.Parallel()

	// Create a metric line > 64KB with many labels
	largeLabelValue := strings.Repeat("x", 70000)
	input := fmt.Sprintf(`# TYPE test_metric gauge
# HELP test_metric Test metric with large label value
test_metric{large_label="%s",namespace="default"} 1.0
# TYPE another_metric counter
# HELP another_metric Another test metric
another_metric{label="value"} 42
`, largeLabelValue)

	reader := strings.NewReader(input)
	logger := logutil.Discard

	// This should fail with the current implementation due to 64KB buffer limit
	filtered, skipped, err := filterUnsupportedMetrics(reader, logger)

	// After the fix, this should succeed
	assert.NoError(t, err, "Should handle large metric lines without error")
	assert.Empty(t, skipped, "No metrics should be skipped")

	// Verify the lines were processed correctly
	filteredContent, err := io.ReadAll(filtered)
	assert.NoError(t, err)
	assert.Contains(t, string(filteredContent), "test_metric")
	assert.Contains(t, string(filteredContent), "another_metric")
	assert.Contains(t, string(filteredContent), largeLabelValue, "Large label value should be preserved")
}

// TestFilterUnsupportedMetrics_MultipleLargeLines tests handling of multiple large metric lines.
func TestFilterUnsupportedMetrics_MultipleLargeLines(t *testing.T) {
	t.Parallel()

	// Create multiple metric lines that exceed 64KB
	largeLabelValue1 := strings.Repeat("a", 70000)
	largeLabelValue2 := strings.Repeat("b", 80000)
	input := fmt.Sprintf(`# TYPE first_metric gauge
# HELP first_metric First test metric
first_metric{large_label="%s"} 1.0
# TYPE second_metric gauge
# HELP second_metric Second test metric
second_metric{large_label="%s"} 2.0
`, largeLabelValue1, largeLabelValue2)

	reader := strings.NewReader(input)
	logger := logutil.Discard

	filtered, skipped, err := filterUnsupportedMetrics(reader, logger)

	assert.NoError(t, err, "Should handle multiple large metric lines without error")
	assert.Empty(t, skipped, "No metrics should be skipped")

	filteredContent, err := io.ReadAll(filtered)
	assert.NoError(t, err)
	assert.Contains(t, string(filteredContent), "first_metric")
	assert.Contains(t, string(filteredContent), "second_metric")
}

// TestFilterUnsupportedMetrics_LargeLinesWithUnsupportedTypes tests that large lines
// work correctly when mixed with unsupported metric types that need to be filtered.
func TestFilterUnsupportedMetrics_LargeLinesWithUnsupportedTypes(t *testing.T) {
	t.Parallel()

	largeLabelValue := strings.Repeat("x", 70000)
	input := fmt.Sprintf(`# TYPE supported_metric gauge
# HELP supported_metric Supported metric with large label
supported_metric{large_label="%s"} 1.0
# TYPE unsupported_metric info
# HELP unsupported_metric This should be filtered out
unsupported_metric{label="value"} 1
# TYPE another_supported_metric counter
# HELP another_supported_metric Another supported metric
another_supported_metric{label="test"} 42
`, largeLabelValue)

	reader := strings.NewReader(input)
	logger := logutil.Discard

	filtered, skipped, err := filterUnsupportedMetrics(reader, logger)

	assert.NoError(t, err, "Should handle large lines with unsupported types")
	assert.Len(t, skipped, 1, "Should skip one unsupported metric")
	assert.Equal(t, "unsupported_metric", skipped[0])

	filteredContent, err := io.ReadAll(filtered)
	assert.NoError(t, err)
	assert.Contains(t, string(filteredContent), "supported_metric")
	assert.Contains(t, string(filteredContent), "another_supported_metric")
	assert.NotContains(t, string(filteredContent), "unsupported_metric")
}
