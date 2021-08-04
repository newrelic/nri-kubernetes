package storage_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

func Test_MemoryStorage(t *testing.T) {
	storageTestSuite(t, func() storage.Storage {
		return &storage.MemoryStorage{}
	})
}

func storageTestSuite(t *testing.T, storageFactory func() storage.Storage) {
	t.Parallel()

	t.Run("writes", func(t *testing.T) {
		t.Parallel()

		t.Run("returns_error_when", func(t *testing.T) {
			t.Parallel()

			t.Run("given_value_is_nil", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				require.NotNil(t, m.Write(testKey, nil))
			})

			t.Run("given_value_is_not_a_pointer", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				v := testStruct{}
				require.NotNil(t, m.Write(testKey, v))
			})
		})
	})

	t.Run("reads", func(t *testing.T) {
		t.Parallel()

		t.Run("returns_error_when", func(t *testing.T) {
			t.Parallel()

			t.Run("given_value_is_nil", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				_, err := m.Read(testKey, nil)
				require.NotNil(t, err)
			})

			t.Run("given_value_is_not_a_pointer", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				v := testStruct{}
				_, err := m.Read(testKey, v)
				require.NotNil(t, err)
			})

			t.Run("given_key_is_not_found", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				v := &testStruct{}
				_, err := m.Read(testKey, v)
				require.NotNil(t, err)
			})
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		t.Run("returns_error_when", func(t *testing.T) {
			t.Parallel()

			t.Run("given_key_is_not_found", func(t *testing.T) {
				t.Parallel()

				m := storageFactory()
				require.NotNil(t, m.Delete(testKey))
			})
		})

		t.Run("removes_value_from_storage", func(t *testing.T) {
			t.Parallel()

			m := storageFactory()
			v := &testStruct{}
			require.Nil(t, m.Write(testKey, v))
			require.Nil(t, m.Delete(testKey))
			require.NotNil(t, m.Delete(testKey))
		})
	})

	t.Run("retains_written_value_of_type_struct", func(t *testing.T) {
		m := storageFactory()

		valueToBeStored := &testStruct{
			foo: "bar",
		}

		require.Nil(t, m.Write(testKey, valueToBeStored), "writing to storage")

		retrievedValue := &testStruct{}

		_, err := m.Read(testKey, retrievedValue)
		require.Nil(t, err, "reading from storage")

		require.True(t, reflect.DeepEqual(valueToBeStored, retrievedValue))
	})
}

const (
	testKey = "foo"
)

type testStruct struct {
	foo string
}
