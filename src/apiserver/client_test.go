package apiserver

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func getTempDir(t *testing.T) (string, func()) {
	tmpDir, err := ioutil.TempDir("", "testrunner")
	require.NoError(t, err, "could not create temporary test directory")

	return tmpDir, func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			logrus.Warningf("Could not remove temporary test directory: %v", err)
		}
	}
}

// TestFileCacheReadMiss tests whether the fileCache will handle missing files on disk
func TestFileCacheReadMiss(t *testing.T) {

	dir, cleanup := getTempDir(t)
	defer cleanup()

	myNode := &NodeInfo{
		NodeName: "MyNode",
		Labels: map[string]string{
			"kubernetes.io/hostname": "MyNode",
			"kubernetes.io/os":       "linux",
		},
		Allocatable: v1.ResourceList{
			v1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
			v1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
			v1.ResourceEphemeralStorage: *resource.NewQuantity(18211580000, resource.BinarySI),
			v1.ResourceMemory:           *resource.NewQuantity(2033280000, resource.BinarySI),
		},
		Capacity: v1.ResourceList{
			v1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
			v1.ResourcePods:             *resource.NewQuantity(110, resource.DecimalSI),
			v1.ResourceEphemeralStorage: *resource.NewQuantity(18211586048, resource.BinarySI),
			v1.ResourceMemory:           *resource.NewQuantity(2033283072, resource.BinarySI),
		},
	}
	client := TestAPIServer{Mem: map[string]*NodeInfo{"MyNode": myNode}}

	cacheWrapper := NewFileCacheClientWrapper(client, dir, time.Hour)

	node, err := cacheWrapper.GetNodeInfo("MyNode")

	assert.NoError(t, err)
	assert.Equal(t, node, myNode)
}

// TestFileCacheReadCacheAndExpiry tests whether the fileCache will properly read from cache, and that it resets the cache
func TestFileCacheReadCacheAndExpiry(t *testing.T) {

	dir, cleanup := getTempDir(t)
	defer cleanup()

	// The resource.Quantity struct has an attribute called `s` that caches
	// the string representation. This attribute is calculated and stored
	// when calling the String and UnmarshalJSON functions. We need to call
	// the String function when creating the Quantities to make the structs
	// match and not fail on the assert.Equal function.
	cpu := resource.NewQuantity(2, resource.DecimalSI)
	_ = cpu.String()
	newCPU := resource.NewQuantity(4, resource.DecimalSI)
	_ = newCPU.String()
	memory := resource.NewQuantity(2033283072, resource.BinarySI)
	_ = memory.String()

	myNode := &NodeInfo{
		NodeName: "MyNode",
		Labels: map[string]string{
			"kubernetes.io/hostname": "MyNode",
			"kubernetes.io/os":       "linux",
		},
		Allocatable: v1.ResourceList{
			v1.ResourceCPU: *cpu,
		},
		Capacity: v1.ResourceList{
			v1.ResourceCPU: *cpu,
		},
	}
	myUpdatedNode := &NodeInfo{
		NodeName: "MyNode",
		Labels: map[string]string{
			"kubernetes.io/hostname": "updatedHostname",
			"kubernetes.io/os":       "linux",
			"kubernetes.io/updated":  "true",
		},
		Allocatable: v1.ResourceList{
			v1.ResourceCPU:    *cpu,
			v1.ResourceMemory: *memory,
		},
		Capacity: v1.ResourceList{
			v1.ResourceCPU:    *newCPU,
			v1.ResourceMemory: *memory,
		},
	}

	client := TestAPIServer{Mem: map[string]*NodeInfo{"MyNode": myNode}}

	timeProvider := &manualTimeProvider{time.Now()}

	cacheWrapper := NewFileCacheClientWrapper(client, dir, time.Hour, WithTimeProvider(timeProvider))

	// this will have written the response to disk
	_, err := cacheWrapper.GetNodeInfo("MyNode")
	assert.NoError(t, err)

	// we update the node in the fake APIServer
	client.Mem["MyNode"] = myUpdatedNode

	// Reading from the cacheWrapper should still return the old version
	node, err := cacheWrapper.GetNodeInfo("MyNode")
	assert.NoError(t, err)

	assert.Equal(t, myNode, node)

	// While reading from the client should return the new version
	node, err = client.GetNodeInfo("MyNode")
	assert.NoError(t, err)
	assert.Equal(t, myUpdatedNode, node)

	// The cache should reset after 1 hour, and the cacheWrapper should return the updated object
	timeProvider.time = time.Now().Add(time.Hour * 2)

	node, err = cacheWrapper.GetNodeInfo("MyNode")
	assert.NoError(t, err)
	assert.Equal(t, node, myUpdatedNode)

}

type manualTimeProvider struct {
	time time.Time
}

func (m manualTimeProvider) Time() time.Time { return m.time }
