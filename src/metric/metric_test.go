package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
