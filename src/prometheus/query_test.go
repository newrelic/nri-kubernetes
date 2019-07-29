package prometheus

import (
	"io"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/stretchr/testify/assert"

	"os"

	"github.com/golang/protobuf/proto"

	model "github.com/prometheus/client_model/go"
)

type ksm struct {
	nodeIP string
}

func (c *ksm) Do(method, path string) (*http.Response, error) {
	f, err := os.Open("testdata/metrics_plain.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint: errcheck

	w := httptest.NewRecorder()

	io.Copy(w, f) // nolint: errcheck

	return w.Result(), nil
}

func (c *ksm) NodeIP() string {
	return c.nodeIP
}

func TestDo(t *testing.T) {
	// TODO create or use an agnostic test sample.
	var c = ksm{
		nodeIP: "1.2.3.4",
	}

	queryMetricName := "kube_pod_status_phase"
	queryLabels := Labels{
		"namespace": "default",
		"pod":       "smoya-ghtop-6878dbdcc4-x2c5f",
	}

	queries := []Query{
		{
			MetricName: queryMetricName,
			Labels: QueryLabels{
				Labels: queryLabels,
			},
			Value: QueryValue{
				Value: GaugeValue(1),
			},
		},
	}

	expectedLabels := queryLabels
	expectedLabels["phase"] = "Running"
	expectedMetrics := []MetricFamily{
		{
			Name: queryMetricName,
			Type: "GAUGE",
			Metrics: []Metric{
				{
					Labels: expectedLabels,
					Value:  GaugeValue(1),
				},
			},
		},
	}

	m, err := Do(&c, "", queries)
	assert.NoError(t, err)

	assert.Equal(t, expectedMetrics, m)
}

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
