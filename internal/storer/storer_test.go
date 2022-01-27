package storer_test

import (
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/nri-kubernetes/v3/internal/storer"
)

const (
	testValue    = float64(5)
	testNewValue = float64(15)
	testKey      = "testKey"
)

func Test_Cache(t *testing.T) {
	t.Parallel()

	t.Run("is_set", func(t *testing.T) {
		t.Parallel()
		var val float64
		cache := storer.NewInMemoryStore(time.Second, time.Second*3, logrus.New())

		cache.Set(testKey, testValue)
		_, err := cache.Get(testKey, &val)

		assert.Equal(t, testValue, val)
		assert.NoError(t, err)

		t.Run("and_overwritten", func(t *testing.T) {
			cache.Set(testKey, testNewValue)
			_, err = cache.Get(testKey, &val)

			assert.Equal(t, testNewValue, val)
			assert.NoError(t, err)
		})

		t.Run("and_after_interval_and_TTL_is_garbage_collected", func(t *testing.T) {
			time.Sleep(time.Second * 4)
			_, err = cache.Get(testKey, &val)
			assert.ErrorIs(t, err, persist.ErrNotFound)
		})
	})

	t.Run("is_set_and_after_interval_but_not_TTL_is_not_garbage_collected", func(t *testing.T) {
		t.Parallel()
		var val float64
		cache := storer.NewInMemoryStore(time.Second*50, time.Second*1, logrus.New())

		cache.Set(testKey, testValue)
		time.Sleep(time.Second * 3)

		_, err := cache.Get(testKey, &val)
		assert.Equal(t, testValue, val)
		assert.NoError(t, err)
	})

	t.Run("fails_if_value_is_nil_or_not_a_pointer", func(t *testing.T) {
		t.Parallel()

		cache := storer.NewInMemoryStore(time.Second*50, time.Second*1, logrus.New())
		_ = cache.Set(testKey, testValue)

		_, err := cache.Get(testKey, nil)
		assert.Error(t, err)
		_, err = cache.Get(testKey, testKey)
		assert.Error(t, err)
	})

	t.Run("fails_if_value_is_a_different_pointer", func(t *testing.T) {
		t.Parallel()

		var val string
		cache := storer.NewInMemoryStore(time.Second*50, time.Second*1, logrus.New())
		_ = cache.Set(testKey, testValue)

		_, err := cache.Get(testKey, &val)
		assert.Error(t, err)
	})

	t.Run("without_a_hit_returns_zero_and_err_not_found", func(t *testing.T) {
		t.Parallel()
		var val float64

		cache := storer.NewInMemoryStore(time.Second*50, time.Second*1, logrus.New())
		timestamp, err := cache.Get(testKey, &val)
		assert.ErrorIs(t, err, persist.ErrNotFound)
		assert.Equal(t, val, float64(0))
		assert.Equal(t, timestamp, int64(0))
	})

	t.Run("cleans_itself_periodically", func(t *testing.T) {
		t.Parallel()
		var val float64
		cache := storer.NewInMemoryStore(time.Millisecond, time.Millisecond*50, logrus.New())

		for i := 0; i < 5; i++ {
			cache.Set(testKey, testValue)
			time.Sleep(time.Millisecond * 200)
			timestamp, err := cache.Get(testKey, &val)

			assert.ErrorIs(t, err, persist.ErrNotFound)
			assert.Equal(t, timestamp, int64(0))
			assert.Equal(t, val, float64(0))
		}
	})

	t.Run("does_not_delete_old_entries_if_stopped", func(t *testing.T) {
		t.Parallel()
		var val float64
		cache := storer.NewInMemoryStore(time.Millisecond, time.Millisecond*50, logrus.New())
		cache.StopVacuum()

		for i := 0; i < 5; i++ {
			cache.Set(testKey, testValue)
			time.Sleep(time.Millisecond * 200)
			_, err := cache.Get(testKey, &val)

			assert.NoError(t, err)
			assert.Equal(t, val, testValue)
		}
	})
}
