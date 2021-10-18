package network

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

func getInterfaceMock(defaultInterface string) defaultInterfaceFunc {
	return func(_ string) (string, error) { return defaultInterface, nil }
}

func TestCachedDefaultInterface(t *testing.T) {
	logger := logrus.StandardLogger()
	cacheStorage := storage.NewJSONDiskStorage(t.TempDir())

	// Get the interface from the defaultInterfaceFunc and cache
	i, err := doCachedDefaultInterface(
		logger,
		getInterfaceMock("eth0"),
		"",
		cacheStorage,
		time.Duration(1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "eth0", i)

	// Changing the return value of the defaultInterfaceFunc returns the
	// cache value because the TTL has not expired.
	i, err = doCachedDefaultInterface(
		logger,
		getInterfaceMock("eth1"),
		"",
		cacheStorage,
		time.Duration(1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "eth0", i)

	// Set the TTL to 0 to expire the cache and get the new interface.
	i, err = doCachedDefaultInterface(
		logger,
		getInterfaceMock("eth1"),
		"",
		cacheStorage,
		time.Duration(0))
	require.NoError(t, err)
	assert.Equal(t, "eth1", i)
}
