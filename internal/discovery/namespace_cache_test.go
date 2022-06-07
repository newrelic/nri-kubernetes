package discovery_test

import (
	"testing"
	"time"

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
		cache := discovery.NewNamespaceInMemoryStore(time.Second, logrus.New())

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

		t.Run("and_after_interval_is_garbage_collected", func(t *testing.T) {
			cache.Put(testKey, testValue)
			time.Sleep(time.Second * 3)
			_, found = cache.Match(testKey)

			require.Equal(t, false, found)
		})
	})

	t.Run("miss_returns_namespace_not_found", func(t *testing.T) {
		t.Parallel()

		cache := discovery.NewNamespaceInMemoryStore(time.Second, logrus.New())
		match, found := cache.Match(testKey)
		require.Equal(t, false, match)
		require.Equal(t, false, found)
	})

	t.Run("does_not_delete_old_entries_if_stopped", func(t *testing.T) {
		t.Parallel()
		cache := discovery.NewNamespaceInMemoryStore(time.Second, logrus.New())
		cache.StopVacuum()

		for i := 0; i < 2; i++ {
			cache.Put(testKey, testValue)
			time.Sleep(time.Millisecond * 200)
			match, _ := cache.Match(testKey)

			require.Equal(t, match, testValue)
		}
	})
}
