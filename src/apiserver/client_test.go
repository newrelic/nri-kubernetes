package apiserver

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	myNode := &NodeInfo{
		NodeName: "MyNode",
		Labels: map[string]string{
			"kubernetes.io/hostname": "MyNode",
			"kubernetes.io/os":       "linux",
		},
	}
	myUpdatedNode := &NodeInfo{
		NodeName: "MyNode",
		Labels: map[string]string{
			"kubernetes.io/hostname": "updatedHostname",
			"kubernetes.io/os":       "linux",
			"kubernetes.io/updated":  "true",
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
	assert.Equal(t, node, myNode)

	// While reading from the client should return the new version
	node, err = client.GetNodeInfo("MyNode")
	assert.NoError(t, err)
	assert.Equal(t, node, myUpdatedNode)

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
