package metric

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

// InterfaceCache caches the resolved network interface name for each entity.
// This avoids repeated file I/O operations and heuristic calculations.
type InterfaceCache struct {
	cache map[string]string // entityID -> interface name
	mu    sync.RWMutex
}

// NewInterfaceCache creates a new interface cache.
func NewInterfaceCache() *InterfaceCache {
	return &InterfaceCache{
		cache: make(map[string]string),
	}
}

// Get retrieves a cached interface name for an entity.
// Returns (interfaceName, found).
func (c *InterfaceCache) Get(entityID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	iface, found := c.cache[entityID]
	return iface, found
}

// Put stores an interface name for an entity.
func (c *InterfaceCache) Put(entityID, interfaceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[entityID] = interfaceName
}

// Vacuum clears the cache. Should be called between scrape iterations.
func (c *InterfaceCache) Vacuum() {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Debugf("Vacuuming interface cache: %d entries", len(c.cache))
	c.cache = make(map[string]string)
}
