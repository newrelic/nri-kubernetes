package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	assert.Equal(t, "1358.289250117", CounterValue(1358.289250117).String())
	assert.Equal(t, "1", CounterValue(1).String())
}

func TestGauge(t *testing.T) {
	assert.Equal(t, "1358.289250117", GaugeValue(1358.289250117).String())
	assert.Equal(t, "1", GaugeValue(1).String())
}
