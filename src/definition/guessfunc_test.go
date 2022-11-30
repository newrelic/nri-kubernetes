package definition

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8sMetricSetTypeGuesser(t *testing.T) {
	t.Parallel()

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
		testCase := testCase
		t.Run(testCase.groupLabel, func(t *testing.T) {
			t.Parallel()

			guess, err := K8sMetricSetTypeGuesser(testCase.groupLabel)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, guess)
		})
	}
}
