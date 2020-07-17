package metric

import (
	"testing"

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
