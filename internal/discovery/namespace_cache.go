package discovery

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultInterval is default interval to execute the "garbage collection" of the cache.
	DefaultInterval = 15 * time.Minute
)

type cachedData map[string]bool

// NamespaceCache provides an interface to add and retrieve namespaces from the cache.
type NamespaceCache interface {
	Put(namespace string, match bool)
	Match(namespace string) (bool, bool)
}

type NamespaceInMemoryStore struct {
	cache       cachedData
	locker      *sync.RWMutex
	logger      *logrus.Logger
	lastVacuum  time.Time
	ticker      *time.Ticker
	stopChannel chan struct{}
}

func NewNamespaceInMemoryStore(interval time.Duration, logger *logrus.Logger) *NamespaceInMemoryStore {
	cm := &NamespaceInMemoryStore{
		cache:  make(cachedData),
		locker: &sync.RWMutex{},
		logger: logger,
		// ticker interval should be slightly smaller than the integration interval.
		ticker:      time.NewTicker(interval),
		stopChannel: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-cm.ticker.C:
				cm.vacuum()
			case <-cm.stopChannel:
				return
			}
		}
	}()

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

// StopVacuum Stops the goroutine in charge of the vacuum of the cache.
func (m *NamespaceInMemoryStore) StopVacuum() {
	m.logger.Debugf("stopping namespace cache vacuum goroutine")
	m.ticker.Stop()
	close(m.stopChannel)
}

// vacuum removes the cached data entries on each interval.
func (m *NamespaceInMemoryStore) vacuum() {
	m.locker.Lock()
	defer m.locker.Unlock()

	m.logger.Debugf("cleaning cache: len %d ...", len(m.cache))
	m.cache = make(cachedData)
	m.logger.Debugf("cache cleaned: len %d ...", len(m.cache))
}
