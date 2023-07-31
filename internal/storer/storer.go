// Package storer is implements a cache deleting its entries after each interval.
package storer

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultTTL is default ttl of the ache entries.
	DefaultTTL = 10 * time.Minute
	// DefaultInterval is default interval to execute the "garbage collection" of the cache.
	DefaultInterval = 15 * time.Minute
)

type Storer interface {
	Set(key string, value interface{}) int64
	Get(key string, valuePtr interface{}) (int64, error)
}

// InMemoryStore is similar to the sdk one, the main difference is cleanCache method executed each interval.
type InMemoryStore struct {
	cachedData  map[string]jsonEntry
	locker      *sync.RWMutex
	ttl         time.Duration
	logger      *logrus.Logger
	ticker      *time.Ticker
	stopChannel chan struct{}
}

// Holder for any entry in the JSON storage.
type jsonEntry struct {
	// notice this is the timestamp of the creation of the jsonEntry, we do not keep track of the last-access timestamp.
	timestamp time.Time
	value     interface{}
}

// NewInMemoryStore will create and initialize an InMemoryStore.
func NewInMemoryStore(ttl time.Duration, interval time.Duration, logger *logrus.Logger) *InMemoryStore {
	ims := &InMemoryStore{
		cachedData:  make(map[string]jsonEntry),
		locker:      &sync.RWMutex{},
		ttl:         ttl,
		logger:      logger,
		ticker:      time.NewTicker(interval),
		stopChannel: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-ims.ticker.C:
				ims.vacuum()
			case <-ims.stopChannel:
				return
			}
		}
	}()

	return ims
}

// Set stores a value for a given key.
func (ims InMemoryStore) Set(key string, value interface{}) int64 {
	ims.locker.Lock()
	defer ims.locker.Unlock()

	ts := time.Now()
	ims.cachedData[key] = jsonEntry{
		timestamp: ts,
		value:     value,
	}
	return ts.Unix()
}

// Get gets the value associated to a given key and stores it in the value referenced by the pointer passed as
// second argument.
func (ims InMemoryStore) Get(key string, valuePtr interface{}) (int64, error) {
	ims.locker.RLock()
	defer ims.locker.RUnlock()

	rv := reflect.ValueOf(valuePtr)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, errors.New("destination argument must be an empty pointer")
	}

	entry, ok := ims.cachedData[key]
	if !ok {
		// Notice that we have to return persist.ErrNotFound, any other error will be interpreted
		// as a generic error exiting. The data would be never set.
		return 0, persist.ErrNotFound
	}

	// Using reflection to indirectly set the value passed as reference
	varToPopulate := reflect.Indirect(reflect.ValueOf(valuePtr))
	valueToSet := reflect.Indirect(reflect.ValueOf(entry.value))

	if !valueToSet.Type().AssignableTo(varToPopulate.Type()) {
		return 0, fmt.Errorf("the types of cache source and dst are different: %q %q", valueToSet.Type(), varToPopulate.Type())
	}
	varToPopulate.Set(valueToSet)

	return entry.timestamp.Unix(), nil
}

// vacuum removes the cached data entries if older than TTL.
func (ims InMemoryStore) vacuum() {
	ims.locker.Lock()
	defer ims.locker.Unlock()

	ims.logger.Debugf("cleaning cache: len %d ...", len(ims.cachedData))
	for k, v := range ims.cachedData {
		if time.Since(v.timestamp).Seconds() > ims.ttl.Seconds() {
			delete(ims.cachedData, k)
		}
	}
	ims.logger.Debugf("cache cleaned: len %d ...", len(ims.cachedData))
}

// StopVacuum Stops the goroutine in charge of the vacuum of the cache.
func (ims InMemoryStore) StopVacuum() {
	ims.logger.Debugf("stopping vacuum goroutine")
	ims.ticker.Stop()
	close(ims.stopChannel)
}

// Save implementation to respect interface.
func (ims *InMemoryStore) Save() error {
	// It does nothing, not needed in this implementation
	return nil
}

// Delete implementation to respect interface.
func (ims InMemoryStore) Delete(_ string) error {
	// It does nothing, not needed in this implementation
	return nil
}
