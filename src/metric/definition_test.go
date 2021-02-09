package metric

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"

	"time"

	"github.com/stretchr/testify/assert"
)

func TestFromNano(t *testing.T) {
	v, err := fromNano(uint64(123456789))
	assert.Equal(t, 0.123456789, v)
	assert.NoError(t, err)

	v, err = fromNano(123456789)
	assert.Nil(t, v)
	assert.Error(t, err)

	v, err = fromNano("not-valid")
	assert.Nil(t, v)
	assert.Error(t, err)
}

func TestFromNanoToMilli(t *testing.T) {
	v, err := fromNanoToMilli(uint64(123456789))
	assert.Equal(t, 123.456789, v)
	assert.NoError(t, err)

	v, err = fromNano(123456789)
	assert.Nil(t, v)
	assert.Error(t, err)

	v, err = fromNano("not-valid")
	assert.Nil(t, v)
	assert.Error(t, err)
}

func TestToTimestap(t *testing.T) {
	t1, _ := time.Parse(time.RFC3339, "2018-02-14T16:26:33Z")
	v, err := toTimestamp(t1)
	assert.Equal(t, int64(1518625593), v)
	assert.NoError(t, err)

	t2, _ := time.Parse(time.RFC3339, "2016-10-21T00:45:12Z")
	v, err = toTimestamp(t2)
	assert.Equal(t, int64(1477010712), v)
	assert.NoError(t, err)
}

func TestToNumericBoolean(t *testing.T) {
	v, err := toNumericBoolean(1)
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(0)
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(true)
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean(false)
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("true")
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("false")
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("True")
	assert.Equal(t, 1, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("False")
	assert.Equal(t, 0, v)
	assert.NoError(t, err)

	v, err = toNumericBoolean("invalid")
	assert.Nil(t, v)
	assert.EqualError(t, err, "value can not be converted to numeric boolean")
}

func TestToCores(t *testing.T) {
	v, err := toCores(100)
	assert.Equal(t, float64(0.1), v)
	assert.NoError(t, err)

	v, err = toCores(int64(1000))
	assert.Equal(t, float64(1), v)
	assert.NoError(t, err)
}

func TestComputePercentage(t *testing.T) {
	v, err := computePercentage(3, 5)
	assert.Equal(t, float64(60.0), v)
	assert.NoError(t, err)

	v, err = computePercentage(3, 0)
	assert.EqualError(t, err, "division by zero")
}

func TestSubtract(t *testing.T) {

	left := definition.FetchFunc(func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
		return prometheus.GaugeValue(10), nil
	})

	right := definition.FetchFunc(func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
		return prometheus.GaugeValue(5), nil
	})

	sub := Subtract(definition.Transform(left, fromPrometheusNumeric), definition.Transform(right, fromPrometheusNumeric))
	result, err := sub("", "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, result, float64(5))
}

func TestUtilization(t *testing.T) {
	raw := definition.RawGroups{
		"group1": {
			"entity1": {
				"dividend": uint64(10),
				"divisor":  uint64(20),
			},
		},
	}

	value, err := toUtilization("dividend", "divisor")("group1", "entity1", raw)
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, float64(50), value)
}
