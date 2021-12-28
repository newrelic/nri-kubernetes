package prometheus

import (
	"testing"

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
