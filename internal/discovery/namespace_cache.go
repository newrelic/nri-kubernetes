package discovery

import (
	"sync"

	"github.com/sirupsen/logrus"
)

type cachedData map[string]bool

// NamespaceCache provides an interface to add and retrieve namespaces from the cache.
type NamespaceCache interface {
	Put(namespace string, match bool)
	Match(namespace string) (bool, bool)
	Vacuum()
}

type NamespaceInMemoryStore struct {
	cache  cachedData
	locker *sync.RWMutex
	logger *logrus.Logger
}

func NewNamespaceInMemoryStore(logger *logrus.Logger) *NamespaceInMemoryStore {
	cm := &NamespaceInMemoryStore{
		cache:  make(cachedData),
		locker: &sync.RWMutex{},
		logger: logger,
	}

	return cm
}

func (m *NamespaceInMemoryStore) Put(namespace string, match bool) {
	m.locker.Lock()
	defer m.locker.Unlock()

	m.cache[namespace] = match
}

func (m *NamespaceInMemoryStore) Match(namespace string) (bool, bool) {
	m.locker.Lock()
	defer m.locker.Unlock()

	match, found := m.cache[namespace]

	return match, found
}

// Vacuum removes the cached data entries on each interval.
func (m *NamespaceInMemoryStore) Vacuum() {
	m.locker.Lock()
	defer m.locker.Unlock()

	m.logger.Debugf("cleaning cache: len %d ...", len(m.cache))
	m.cache = make(cachedData)
	m.logger.Debugf("cache cleaned: len %d ...", len(m.cache))
}
