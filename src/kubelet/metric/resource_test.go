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
	testGroupLabel = "node"
	testEntityID   = "test-node"
	testRawKey     = string(resourceTypeAllocatable)
)

func rawGroupsWithAllocatable(allocatable v1.ResourceList) definition.RawGroups {
	return definition.RawGroups{
		testGroupLabel: {
			testEntityID: definition.RawMetrics{
				testRawKey: allocatable,
			},
		},
	}
}

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

func TestAllocatableCPUCores(t *testing.T) {
	t.Parallel()

	t.Run("returns cpu cores as float64", func(t *testing.T) {
		t.Parallel()
		raw := rawGroupsWithAllocatable(v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("250m"),
		})
		val, err := AllocatableCPUCores()(testGroupLabel, testEntityID, raw)
		require.NoError(t, err)
		assert.InDelta(t, 0.25, val, 0.0001)
	})

	t.Run("uses AsApproximateFloat64 to avoid rounding", func(t *testing.T) {
		t.Parallel()
		raw := rawGroupsWithAllocatable(v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("1985m"),
		})
		val, err := AllocatableCPUCores()(testGroupLabel, testEntityID, raw)
		require.NoError(t, err)
		assert.InDelta(t, 1.985, val, 0.0001)
	})

	t.Run("errors when cpu not present in allocatable", func(t *testing.T) {
		t.Parallel()
		raw := rawGroupsWithAllocatable(v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("512Mi"),
		})
		val, err := AllocatableCPUCores()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})

	t.Run("errors when allocatable key missing from raw groups", func(t *testing.T) {
		t.Parallel()
		raw := definition.RawGroups{
			testGroupLabel: {testEntityID: definition.RawMetrics{}},
		}
		val, err := AllocatableCPUCores()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})

	t.Run("errors when allocatable value is wrong type", func(t *testing.T) {
		t.Parallel()
		raw := definition.RawGroups{
			testGroupLabel: {testEntityID: definition.RawMetrics{testRawKey: "not-a-resource-list"}},
		}
		val, err := AllocatableCPUCores()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})
}

func TestAllocatableMemoryBytes(t *testing.T) {
	t.Parallel()

	t.Run("returns memory as int64 bytes", func(t *testing.T) {
		t.Parallel()
		raw := rawGroupsWithAllocatable(v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("512Mi"),
		})
		val, err := AllocatableMemoryBytes()(testGroupLabel, testEntityID, raw)
		require.NoError(t, err)
		assert.Equal(t, int64(512*1024*1024), val)
	})

	t.Run("errors when memory not present in allocatable", func(t *testing.T) {
		t.Parallel()
		raw := rawGroupsWithAllocatable(v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("250m"),
		})
		val, err := AllocatableMemoryBytes()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})

	t.Run("errors when allocatable key missing from raw groups", func(t *testing.T) {
		t.Parallel()
		raw := definition.RawGroups{
			testGroupLabel: {testEntityID: definition.RawMetrics{}},
		}
		val, err := AllocatableMemoryBytes()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})

	t.Run("errors when allocatable value is wrong type", func(t *testing.T) {
		t.Parallel()
		raw := definition.RawGroups{
			testGroupLabel: {testEntityID: definition.RawMetrics{testRawKey: "not-a-resource-list"}},
		}
		val, err := AllocatableMemoryBytes()(testGroupLabel, testEntityID, raw)
		assert.Error(t, err)
		assert.Nil(t, val)
	})
}
