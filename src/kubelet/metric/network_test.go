package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

func TestFromRawWithFallbackToDefaultInterface_UsesRaw(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"name":    "",
				"rxBytes": uint64(51419684038),
			},
		},
		"network": {
			"interfaces": {
				"default": "thisIsTheDefault",
			},
		},
	}

	f := FromRawWithFallbackToDefaultInterface("rxBytes")
	valueI, err := f("node", "fooNode", expectedRawData)
	require.NoError(t, err)

	value, ok := valueI.(uint64)
	require.True(t, ok)
	assert.Equal(t, uint64(51419684038), value)
}

func TestFromRawWithFallbackToDefaultInterface_UsesFallback(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"name": "",
				"interfaces": map[string]definition.RawMetrics{
					"thisIsTheDefault": {
						"rxBytes": uint64(51419684038),
						"txBytes": uint64(25630208577),
						"errors":  uint64(0),
					},
				},
			},
		},
		"network": {
			"interfaces": {
				"default": "thisIsTheDefault",
			},
		},
	}

	f := FromRawWithFallbackToDefaultInterface("rxBytes")
	valueI, err := f("node", "fooNode", expectedRawData)
	require.NoError(t, err)

	value, ok := valueI.(uint64)
	require.True(t, ok)
	assert.Equal(t, uint64(51419684038), value)
}
