package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInterfaceCache tests the cache Get/Put/Vacuum operations.
func TestInterfaceCache(t *testing.T) {
	t.Parallel()

	t.Run("Get returns false for non-existent key", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()
		iface, found := cache.Get("non-existent")
		assert.False(t, found)
		assert.Equal(t, "", iface)
	})

	t.Run("Put and Get work correctly", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()

		cache.Put("entity1", "eth0")
		iface, found := cache.Get("entity1")
		assert.True(t, found)
		assert.Equal(t, "eth0", iface)
	})

	t.Run("Put overwrites existing value", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()

		cache.Put("entity1", "eth0")
		cache.Put("entity1", "eth1")

		iface, found := cache.Get("entity1")
		assert.True(t, found)
		assert.Equal(t, "eth1", iface)
	})

	t.Run("Multiple entities can be cached", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()

		cache.Put("entity1", "eth0")
		cache.Put("entity2", "ens3")
		cache.Put("entity3", "ens5")

		iface1, found1 := cache.Get("entity1")
		assert.True(t, found1)
		assert.Equal(t, "eth0", iface1)

		iface2, found2 := cache.Get("entity2")
		assert.True(t, found2)
		assert.Equal(t, "ens3", iface2)

		iface3, found3 := cache.Get("entity3")
		assert.True(t, found3)
		assert.Equal(t, "ens5", iface3)
	})

	t.Run("Vacuum clears all entries", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()

		cache.Put("entity1", "eth0")
		cache.Put("entity2", "ens3")
		cache.Put("entity3", "ens5")

		// Verify entries exist
		_, found := cache.Get("entity1")
		assert.True(t, found)

		// Vacuum the cache
		cache.Vacuum()

		// Verify all entries are gone
		_, found1 := cache.Get("entity1")
		assert.False(t, found1)

		_, found2 := cache.Get("entity2")
		assert.False(t, found2)

		_, found3 := cache.Get("entity3")
		assert.False(t, found3)
	})

	t.Run("Cache works after Vacuum", func(t *testing.T) {
		t.Parallel()
		cache := NewInterfaceCache()

		cache.Put("entity1", "eth0")
		cache.Vacuum()

		// Should be able to add entries after vacuum
		cache.Put("entity2", "ens3")
		iface, found := cache.Get("entity2")
		assert.True(t, found)
		assert.Equal(t, "ens3", iface)
	})
}
