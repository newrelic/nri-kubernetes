package discovery_test

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	testValue    = true
	testNewValue = false
	testKey      = "test_namespace"
)

func Test_NamespaceCache(t *testing.T) {
	t.Parallel()

	t.Run("is_set", func(t *testing.T) {
		t.Parallel()
		cache := discovery.NewNamespaceInMemoryStore(logrus.New())

		cache.Put(testKey, testValue)
		match, found := cache.Match(testKey)

		require.Equal(t, testValue, match)
		require.Equal(t, true, found)

		t.Run("and_overwritten", func(t *testing.T) {
			cache.Put(testKey, testNewValue)
			match, found = cache.Match(testKey)

			require.Equal(t, testNewValue, match)
			require.Equal(t, true, found)
		})

		t.Run("and_after_vacuum_is_garbage_collected", func(t *testing.T) {
			cache.Put(testKey, testValue)
			cache.Vacuum()
			_, found = cache.Match(testKey)

			require.Equal(t, false, found)
		})
	})

	t.Run("miss_returns_namespace_not_found", func(t *testing.T) {
		t.Parallel()

		cache := discovery.NewNamespaceInMemoryStore(logrus.New())
		match, found := cache.Match(testKey)
		require.Equal(t, false, match)
		require.Equal(t, false, found)
	})
}
