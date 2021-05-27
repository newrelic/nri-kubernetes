package metric

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

const (
	// Copied from old version of k8s.io/api/core/v1.
	//
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	resourceNvidiaGPU v1.ResourceName = "alpha.kubernetes.io/nvidia-gpu"
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
		v1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
		v1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
		v1.ResourceEphemeralStorage: *resource.NewQuantity(18211580000, resource.BinarySI),
		v1.ResourceStorage:          *resource.NewQuantity(18211580000, resource.BinarySI),
		v1.ResourceMemory:           *resource.NewQuantity(2033280000, resource.BinarySI),
		resourceNvidiaGPU:           *resource.NewQuantity(42, resource.DecimalSI),
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.resourceType), func(t *testing.T) {
			expected := definition.FetchedValues{
				fmt.Sprintf("%sCpuCores", testCase.resourceType):                   int64(2),
				fmt.Sprintf("%sPods", testCase.resourceType):                       int64(110),
				fmt.Sprintf("%sEphemeralStorageBytes", testCase.resourceType):      int64(18211580000),
				fmt.Sprintf("%sStorageBytes", testCase.resourceType):               int64(18211580000),
				fmt.Sprintf("%sMemoryBytes", testCase.resourceType):                int64(2033280000),
				fmt.Sprintf("%sAlphaKubernetesIoNvidiaGpu", testCase.resourceType): int64(42),
			}

			transformed, err := testCase.transformFunc(rawResources)
			require.NoError(t, err)
			assert.Equal(t, expected, transformed)
		})
	}
}
