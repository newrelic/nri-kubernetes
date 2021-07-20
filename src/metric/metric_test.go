package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

func TestK8sMetricSetTypeGuesser(t *testing.T) {
	testCases := []struct {
		groupLabel string
		expected   string
	}{
		{groupLabel: "replicaset", expected: "K8sReplicasetSample"},
		{groupLabel: "api-server", expected: "K8sApiServerSample"},
		{groupLabel: "controller-manager", expected: "K8sControllerManagerSample"},
		{groupLabel: "-controller-manager-", expected: "K8sControllerManagerSample"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.groupLabel, func(*testing.T) {
			guess, err := K8sMetricSetTypeGuesser("", testCase.groupLabel, "", nil)
			assert.Nil(t, err)
			assert.Equal(t, testCase.expected, guess)
		})
	}
}

func TestSubtractorFunc(t *testing.T) {
	// given 2 FetchFunc
	var left definition.FetchFunc
	var right definition.FetchFunc
	left = func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return float64(10), nil
	}
	right = func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return float64(5), nil
	}

	sub := Subtract(left, right)
	result, err := sub("", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, float64(5), result)
}
