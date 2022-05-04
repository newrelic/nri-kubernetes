package metric

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	resourceTestBeta  v1.ResourceName = "beta.newrelic.com/test-name"
	resourceTestAlpha v1.ResourceName = "alpha.newrelic.com/test-name"
)

func TestOneAttributePerResource(t *testing.T) {
	testCases := []struct {
		resourceType  resourceType
		transformFunc definition.TransformFunc
	}{
		{
			resourceType:  resourceTypeAllocatable,
			transformFunc: OneAttributePerAllocatable,
		},
		{
			resourceType:  resourceTypeCapacity,
			transformFunc: OneAttributePerCapacity,
		},
	}

	rawResources := v1.ResourceList{
		v1.ResourceCPU:                    resource.MustParse("1985m"),
		v1.ResourcePods:                   *resource.NewQuantity(110, resource.DecimalSI),
		v1.ResourceEphemeralStorage:       *resource.NewQuantity(18211580000, resource.BinarySI),
		v1.ResourceStorage:                *resource.NewQuantity(18211580000, resource.BinarySI),
		v1.ResourceMemory:                 *resource.NewQuantity(2033280000, resource.BinarySI),
		v1.ResourceReplicationControllers: *resource.NewQuantity(1, resource.DecimalSI),
		resourceTestBeta:                  resource.MustParse("10985m"),
		resourceTestAlpha:                 *resource.NewQuantity(111, resource.BinarySI),
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.resourceType), func(t *testing.T) {
			expected := definition.FetchedValues{
				fmt.Sprintf("%sCpuCores", testCase.resourceType):                 float64(1.985),
				fmt.Sprintf("%sPods", testCase.resourceType):                     int64(110),
				fmt.Sprintf("%sEphemeralStorageBytes", testCase.resourceType):    int64(18211580000),
				fmt.Sprintf("%sStorageBytes", testCase.resourceType):             int64(18211580000),
				fmt.Sprintf("%sMemoryBytes", testCase.resourceType):              int64(2033280000),
				fmt.Sprintf("%sReplicationcontrollers", testCase.resourceType):   int64(1),
				fmt.Sprintf("%sBetaNewrelicComTestName", testCase.resourceType):  int64(11),
				fmt.Sprintf("%sAlphaNewrelicComTestName", testCase.resourceType): int64(111),
			}

			transformed, err := testCase.transformFunc(rawResources)
			require.NoError(t, err)
			assert.Equal(t, expected, transformed)
		})
	}
}
