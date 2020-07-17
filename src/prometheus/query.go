package prometheus

import (
	"fmt"
	"net/http"

	"github.com/newrelic/nri-kubernetes/src/client"
	model "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
)

//TODO: See https://github.com/prometheus/prom2json/blob/master/prom2json.go#L171 for how to connect, how to parse plain text, etc

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
		// Not supported yet
		fallthrough
	case model.MetricType_UNTYPED:
		// Not supported yet
		fallthrough
	default:
		return EmptyValue
	}
}

// Do is the main entry point. It runs queries against the Prometheus metrics provided by the endpoint.
func Do(c client.HTTPClient, endpoint string, queries []Query) ([]MetricFamily, error) {
	resp, err := c.Do(http.MethodGet, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error calling prometheus exposed metrics endpoint. Got status code: %d", resp.StatusCode)
	}

	metrics := make([]MetricFamily, 0)
	ch := make(chan *model.MetricFamily)

	go func() {
		err = prom2json.ParseResponse(resp, ch)
	}()

	for promMetricFamily := range ch {
		for _, q := range queries {
			f := q.Execute(promMetricFamily)
			if f.Valid() {
				metrics = append(metrics, q.Execute(promMetricFamily))
			}
		}
	}

	return metrics, err
}

func labelsFromPrometheus(pairs []*model.LabelPair) Labels {
	labels := make(Labels)
	for _, p := range pairs {
		labels[p.GetName()] = p.GetValue()
	}

	return labels
}
