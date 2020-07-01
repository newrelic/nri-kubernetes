package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
)

var now = time.Now

// SetNow forces a different "current time" for the cache.
// This function is useful only for unit testing.
func SetNow(newNow func() time.Time) {
	now = newNow
}

const cacheTTL = 1 * time.Minute

// Cache is a map-like structure that is initialized and stored in a JSON
// file. It also saves the timestamp when a key was stored.
type Cache struct {
	path       string
	Data       map[string]interface{}
	Timestamps map[string]int64
}

// NewCache will create and initialize a DiskCache object. It expects the
// NRIA_CACHE_PATH environment variable to point to the file with the cache. If
// it is not set, this will act as a memory-only cache.
func NewCache() (*Cache, error) {
	cache := &Cache{
		Data:       make(map[string]interface{}, 0),
		Timestamps: make(map[string]int64, 0),
	}

	cachePath := os.Getenv("NRIA_CACHE_PATH")
	if cachePath == "" {
		_, fname := filepath.Split(os.Args[0])
		cachePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.json", fname))
		log.Warn("Environment variable NRIA_CACHE_PATH is not set, using default %s", cachePath)
	}

	cache.path = cachePath

	// Create the external directory for user-generated json
	cacheDir := filepath.Dir(cache.path)
	if _, err := os.Stat(cacheDir); err != nil {
		if err = os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("Cache directory in %s could not be created", cacheDir)
		}
	}

	stat, err := os.Stat(cache.path)
	// Cache file doesn't exist yet
	if err != nil {
		return cache, nil
	}

	if now().Sub(stat.ModTime()) > cacheTTL {
		log.Warn(fmt.Sprintf("Cache file (%s) is older than %v, skipping loading from disk.", cachePath, cacheTTL))
		return cache, nil
	}

	file, err := ioutil.ReadFile(cache.path)
	if err != nil {
		log.Warn(fmt.Sprintf("Cache file (%s) cannot be open for reading.", cachePath))
		return cache, nil
	}
	json.Unmarshal(file, &cache)

	return cache, nil
}

// Save marshalls and stores the data a Cache is holding into disk as a JSON
// file.
func (cache *Cache) Save() error {
	if cache.path == "" {
		return nil
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(cache.path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Get looks for a key in the cache and returns its value together with the
// timestamp of when it was last set. The third returned value indicates whether
// the key has been found or not.
func (cache *Cache) Get(name string) (float64, int64, bool) {
	val, ok := cache.Data[name]
	if ok {
		ts, ok := cache.Timestamps[name]
		if ok {
			return val.(float64), int64(ts), ok
		}
	}
	return 0, 0, false
}

// Set adds a value into the cache and it also stores the current timestamp.
func (cache *Cache) Set(name string, value float64) int64 {
	cache.Data[name] = value
	cache.Timestamps[name] = now().Unix()
	return cache.Timestamps[name]
}
