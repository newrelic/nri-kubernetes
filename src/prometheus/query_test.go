package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
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

func TestQuery_Execute_PrefixMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		query            Query
		metricFamilyName string
		expectMatch      bool
	}{
		{
			name: "prefix match - exact prefix",
			query: Query{
				MetricName: "kube_customresource",
				Prefix:     true,
			},
			metricFamilyName: "kube_customresource",
			expectMatch:      true,
		},
		{
			name: "prefix match - with suffix",
			query: Query{
				MetricName: "kube_customresource",
				Prefix:     true,
			},
			metricFamilyName: "kube_customresource_nodepool_limit_cpu",
			expectMatch:      true,
		},
		{
			name: "prefix match - no match",
			query: Query{
				MetricName: "kube_customresource",
				Prefix:     true,
			},
			metricFamilyName: "kube_pod_status_phase",
			expectMatch:      false,
		},
		{
			name: "exact match - matches",
			query: Query{
				MetricName: "kube_pod_status_phase",
				Prefix:     false,
			},
			metricFamilyName: "kube_pod_status_phase",
			expectMatch:      true,
		},
		{
			name: "exact match - no match with similar name",
			query: Query{
				MetricName: "kube_pod_status",
				Prefix:     false,
			},
			metricFamilyName: "kube_pod_status_phase",
			expectMatch:      false,
		},
		{
			name: "prefix match - empty prefix",
			query: Query{
				MetricName: "",
				Prefix:     true,
			},
			metricFamilyName: "any_metric",
			expectMatch:      true, // Empty string is prefix of everything
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a mock metric family
			metricFamily := &model.MetricFamily{
				Name: proto.String(tt.metricFamilyName),
				Type: model.MetricType_GAUGE.Enum(),
				Metric: []*model.Metric{
					{
						Gauge: &model.Gauge{
							Value: proto.Float64(1.0),
						},
					},
				},
			}

			// Execute the query
			result := tt.query.Execute(metricFamily)

			// Check if match occurred
			if tt.expectMatch {
				assert.NotEmpty(t, result.Metrics, "Expected metrics to match but got none")
				assert.Equal(t, tt.metricFamilyName, result.Name, "Expected metric family name to match")
			} else {
				assert.Empty(t, result.Metrics, "Expected no metrics to match but got some")
			}
		})
	}
}

func TestQuery_Execute_PrefixMatchWithLabels(t *testing.T) {
	t.Parallel()

	// Test that prefix matching works correctly with label filtering
	query := Query{
		MetricName: "kube_customresource",
		Prefix:     true,
		Labels: QueryLabels{
			Operator: QueryOpAnd,
			Labels: Labels{
				"name": "default",
			},
		},
	}

	t.Run("prefix match with matching labels", func(t *testing.T) {
		t.Parallel()

		metricFamily := &model.MetricFamily{
			Name: proto.String("kube_customresource_nodepool_limit_cpu"),
			Type: model.MetricType_GAUGE.Enum(),
			Metric: []*model.Metric{
				{
					Label: []*model.LabelPair{
						{
							Name:  proto.String("name"),
							Value: proto.String("default"),
						},
						{
							Name:  proto.String("customresource_group"),
							Value: proto.String("karpenter.sh"),
						},
					},
					Gauge: &model.Gauge{
						Value: proto.Float64(1000.0),
					},
				},
			},
		}

		result := query.Execute(metricFamily)
		assert.Len(t, result.Metrics, 1, "Expected one metric to match")
		assert.Equal(t, "default", result.Metrics[0].Labels["name"])
	})

	t.Run("prefix match with non-matching labels", func(t *testing.T) {
		t.Parallel()

		metricFamily := &model.MetricFamily{
			Name: proto.String("kube_customresource_nodepool_limit_cpu"),
			Type: model.MetricType_GAUGE.Enum(),
			Metric: []*model.Metric{
				{
					Label: []*model.LabelPair{
						{
							Name:  proto.String("name"),
							Value: proto.String("other"),
						},
					},
					Gauge: &model.Gauge{
						Value: proto.Float64(1000.0),
					},
				},
			},
		}

		result := query.Execute(metricFamily)
		assert.Empty(t, result.Metrics, "Expected no metrics to match due to label mismatch")
	})
}

func TestQuery_Execute_PrefixMatchWithValue(t *testing.T) {
	t.Parallel()

	// Test that prefix matching works correctly with value filtering
	query := Query{
		MetricName: "kube_customresource",
		Prefix:     true,
		Value: QueryValue{
			Operator: QueryOpAnd,
			Value:    GaugeValue(1),
		},
	}

	t.Run("prefix match with matching value", func(t *testing.T) {
		t.Parallel()

		metricFamily := &model.MetricFamily{
			Name: proto.String("kube_customresource_test"),
			Type: model.MetricType_GAUGE.Enum(),
			Metric: []*model.Metric{
				{
					Gauge: &model.Gauge{
						Value: proto.Float64(1.0),
					},
				},
			},
		}

		result := query.Execute(metricFamily)
		assert.Len(t, result.Metrics, 1, "Expected one metric to match")
	})

	t.Run("prefix match with non-matching value", func(t *testing.T) {
		t.Parallel()

		metricFamily := &model.MetricFamily{
			Name: proto.String("kube_customresource_test"),
			Type: model.MetricType_GAUGE.Enum(),
			Metric: []*model.Metric{
				{
					Gauge: &model.Gauge{
						Value: proto.Float64(2.0),
					},
				},
			},
		}

		result := query.Execute(metricFamily)
		assert.Empty(t, result.Metrics, "Expected no metrics to match due to value mismatch")
	})
}
