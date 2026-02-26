package metric

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPermissionCache_Basic(t *testing.T) {
	t.Parallel()

	cache := NewPermissionCache(5 * time.Minute)

	// Initially unknown
	assert.Equal(t, PermissionUnknown, cache.Check("/flagz"))
	assert.False(t, cache.IsDenied("/flagz"))

	// Set allowed
	cache.SetAllowed("/flagz")
	assert.Equal(t, PermissionAllowed, cache.Check("/flagz"))
	assert.False(t, cache.IsDenied("/flagz"))

	// Set denied
	cache.SetDenied("/flags", "403 Forbidden")
	assert.Equal(t, PermissionDenied, cache.Check("/flags"))
	assert.True(t, cache.IsDenied("/flags"))
	assert.Equal(t, "403 Forbidden", cache.GetDeniedMessage("/flags"))
}

func TestPermissionCache_TTLExpiry(t *testing.T) {
	t.Parallel()

	// Use very short TTL for testing.
	cache := NewPermissionCache(50 * time.Millisecond)

	cache.SetDenied("/flagz", "403 Forbidden")
	assert.True(t, cache.IsDenied("/flagz"))

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should be unknown after expiry
	assert.Equal(t, PermissionUnknown, cache.Check("/flagz"))
	assert.False(t, cache.IsDenied("/flagz"))
}

func TestPermissionCache_DisabledWithZeroTTL(t *testing.T) {
	t.Parallel()

	cache := NewPermissionCache(0)

	// With zero TTL, caching is disabled
	cache.SetDenied("/flagz", "403 Forbidden")
	assert.Equal(t, PermissionUnknown, cache.Check("/flagz"))
	assert.False(t, cache.IsDenied("/flagz"))
}

func TestPermissionCache_NilSafe(t *testing.T) {
	t.Parallel()

	var cache *PermissionCache

	// Should not panic
	assert.Equal(t, PermissionUnknown, cache.Check("/flagz"))
	assert.False(t, cache.IsDenied("/flagz"))
	cache.SetAllowed("/flagz")
	cache.SetDenied("/flags", "test")
	cache.Clear()
	assert.Equal(t, time.Duration(0), cache.TTL())
}

func TestPermissionCache_Clear(t *testing.T) {
	t.Parallel()

	cache := NewPermissionCache(5 * time.Minute)

	cache.SetDenied("/flagz", "403 Forbidden")
	cache.SetDenied("/flags", "403 Forbidden")
	assert.True(t, cache.IsDenied("/flagz"))
	assert.True(t, cache.IsDenied("/flags"))

	cache.Clear()

	assert.Equal(t, PermissionUnknown, cache.Check("/flagz"))
	assert.Equal(t, PermissionUnknown, cache.Check("/flags"))
}

func TestPermissionCache_MultipleEndpoints(t *testing.T) {
	t.Parallel()

	cache := NewPermissionCache(5 * time.Minute)

	cache.SetAllowed("/configz")
	cache.SetDenied("/flagz", "403")
	cache.SetDenied("/statusz", "404")

	assert.Equal(t, PermissionAllowed, cache.Check("/configz"))
	assert.Equal(t, PermissionDenied, cache.Check("/flagz"))
	assert.Equal(t, PermissionDenied, cache.Check("/statusz"))
	assert.Equal(t, PermissionUnknown, cache.Check("/metrics")) // Not set
}
