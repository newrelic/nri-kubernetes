package prometheus

import (
	"fmt"
	"strconv"

	model "github.com/prometheus/client_model/go"
)

// Value is the value of a metric.
type Value interface {
	fmt.Stringer
}

type noValueType string

func (v noValueType) String() string {
	return string(v)
}

// EmptyValue means we could not get the value.
const EmptyValue noValueType = "no_value"

// Labels is a map containing the label pair of a metric.
type Labels map[string]string

// AreIn says if the labels are included in to the provided Prometheus label pair.
func (l Labels) AreIn(p []*model.LabelPair) bool {
	var votes int
	for name, value := range l {
		for _, pl := range p {
			if pl.GetName() == name && pl.GetValue() == value {
				votes++
			}
		}
	}

	return votes == len(l)
}

// Has checks if the label exists.
func (l Labels) Has(name string) bool {
	if _, ok := l[name]; ok {
		return true
	}

	return false
}

// Metric is for all "single value" metrics, i.e. Counter, Gauge, and Untyped.
type Metric struct {
	Labels Labels
	Value  Value
}

// MetricFamily is an aggregation of metrics with same name.
type MetricFamily struct {
	Name    string
	Type    string
	Metrics []Metric
}

// valid validates that all the attributes were filled.
func (f *MetricFamily) valid() bool {
	return f.Name != "" && f.Type != "" && len(f.Metrics) > 0
}

// CounterValue represents the value of a counter type metric.
type CounterValue float64

// String implements the Stringer interface method.
func (v CounterValue) String() string {
	return strconv.FormatFloat(float64(v), 'f', -1, 64)
}

// GaugeValue represents the value of a gauge type metric.
type GaugeValue float64

// String implements the Stringer interface method.
func (v GaugeValue) String() string {
	return strconv.FormatFloat(float64(v), 'f', -1, 64)
}

// UntypedValue type represents untyped metric values.
type UntypedValue float64

// String implements the Stringer interface method.
func (v UntypedValue) String() string {
	return strconv.FormatFloat(float64(v), 'f', -1, 64)
}
