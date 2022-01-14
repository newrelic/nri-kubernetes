// Package storer is implements a cache deleting its entries after each interval.
package storer

import (
	"errors"
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

// InMemoryStore is similar to the sdk one, the main difference is cleanCache method executed each interval.
type InMemoryStore struct {
	cachedData map[string]jsonEntry
	locker     sync.Locker
	ttl        time.Duration
	logger     *logrus.Logger
}

// Holder for any entry in the JSON storage.
type jsonEntry struct {
	//notice this is the timestamp of the creation of the jsonEntry, we do not keep track of the last-access timestamp.
	timestamp time.Time
	value     interface{}
}

// NewInMemoryStore will create and initialize an InMemoryStore.
func NewInMemoryStore(ttl time.Duration, interval time.Duration, logger *logrus.Logger) *InMemoryStore {
	ims := &InMemoryStore{
		cachedData: make(map[string]jsonEntry),
		locker:     &sync.Mutex{},
		ttl:        ttl,
		logger:     logger,
	}

	tk := time.NewTicker(interval)

	go func() {
		for {
			<-tk.C
			ims.cleanCache()
		}
	}()

	return ims
}

// Set stores a value for a given key.
func (j InMemoryStore) Set(key string, value interface{}) int64 {
	j.locker.Lock()
	defer j.locker.Unlock()

	ts := time.Now()
	j.cachedData[key] = jsonEntry{
		timestamp: ts,
		value:     value,
	}
	return ts.Unix()
}

// Get gets the value associated to a given key and stores it in the value referenced by the pointer passed as
// second argument.
func (j InMemoryStore) Get(key string, valuePtr interface{}) (int64, error) {
	j.locker.Lock()
	defer j.locker.Unlock()

	rv := reflect.ValueOf(valuePtr)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, errors.New("destination argument must be an empty pointer")
	}

	entry, ok := j.cachedData[key]
	if !ok {
		// Notice that we have to return persist.ErrNotFound, any other error will be interpreted
		// as a generic error exiting. The data would be never set.
		return 0, persist.ErrNotFound
	}

	// Using reflection to indirectly set the value passed as reference
	reflect.Indirect(rv).Set(reflect.Indirect(reflect.ValueOf(entry.value)))

	return entry.timestamp.Unix(), nil
}

// cleanCache removes the cached data entries if older than TTL.
func (j InMemoryStore) cleanCache() {
	j.locker.Lock()
	defer j.locker.Unlock()

	j.logger.Errorf("cleaning cache: len %d ...", len(j.cachedData))
	for k, v := range j.cachedData {
		if time.Since(v.timestamp).Seconds() > j.ttl.Seconds() {
			delete(j.cachedData, k)
		}
	}
	j.logger.Errorf("cache cleaned: len %d ...", len(j.cachedData))
}

//Save implementation to respect interface.
func (j *InMemoryStore) Save() error {
	// It does nothing, not needed in this implementation
	return nil
}

// Delete implementation to respect interface.
func (j InMemoryStore) Delete(_ string) error {
	// It does nothing, not needed in this implementation
	return nil
}
