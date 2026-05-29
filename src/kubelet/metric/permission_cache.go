package metric

import (
	"sync"
	"time"
)

// PermissionStatus represents the cached permission status for an endpoint.
type PermissionStatus int

const (
	// PermissionUnknown means the permission hasn't been checked yet.
	PermissionUnknown PermissionStatus = iota
	// PermissionAllowed means the endpoint is accessible.
	PermissionAllowed
	// PermissionDenied means the endpoint returned 403 Forbidden.
	PermissionDenied
)

// PermissionCache caches permission check results for kubelet diagnostic endpoints.
// This prevents repeatedly hitting endpoints that require RBAC permissions we don't have.
type PermissionCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]*permissionEntry
}

type permissionEntry struct {
	status    PermissionStatus
	checkedAt time.Time
	message   string // Optional message explaining why permission was denied
}

// NewPermissionCache creates a new permission cache with the specified TTL.
// If ttl is 0, caching is disabled and all checks return PermissionUnknown.
func NewPermissionCache(ttl time.Duration) *PermissionCache {
	return &PermissionCache{
		ttl:     ttl,
		entries: make(map[string]*permissionEntry),
	}
}

// Check returns the cached permission status for an endpoint.
// Returns PermissionUnknown if:
// - Caching is disabled (ttl=0).
// - The entry doesn't exist.
// - The entry has expired.
func (c *PermissionCache) Check(endpoint string) PermissionStatus {
	if c == nil || c.ttl == 0 {
		return PermissionUnknown
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[endpoint]
	if !ok {
		return PermissionUnknown
	}

	// Check if entry has expired
	if time.Since(entry.checkedAt) > c.ttl {
		return PermissionUnknown
	}

	return entry.status
}

// SetAllowed marks an endpoint as accessible.
func (c *PermissionCache) SetAllowed(endpoint string) {
	if c == nil || c.ttl == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[endpoint] = &permissionEntry{
		status:    PermissionAllowed,
		checkedAt: time.Now(),
	}
}

// SetDenied marks an endpoint as forbidden (403).
func (c *PermissionCache) SetDenied(endpoint string, message string) {
	if c == nil || c.ttl == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[endpoint] = &permissionEntry{
		status:    PermissionDenied,
		checkedAt: time.Now(),
		message:   message,
	}
}

// IsDenied returns true if the endpoint is known to be forbidden.
func (c *PermissionCache) IsDenied(endpoint string) bool {
	return c.Check(endpoint) == PermissionDenied
}

// GetDeniedMessage returns the denial message for an endpoint, if any.
func (c *PermissionCache) GetDeniedMessage(endpoint string) string {
	if c == nil || c.ttl == 0 {
		return ""
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[endpoint]
	if !ok {
		return ""
	}

	return entry.message
}

// Clear removes all cached entries.
func (c *PermissionCache) Clear() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*permissionEntry)
}

// TTL returns the cache TTL.
func (c *PermissionCache) TTL() time.Duration {
	if c == nil {
		return 0
	}
	return c.ttl
}
