package apiserver

import (
	"fmt"
	"reflect"
	"time"

	"github.com/newrelic/nri-kubernetes/src/storage"
)

// Option is a func that configures the fileCacheClient
type Option func(*fileCacheClient)

// WithTimeProvider overrides the default TimeProvider (which returns the current time).
// This is useful for testing cache durations/evictions
func WithTimeProvider(timeProvider TimeProvider) Option {
	return func(f *fileCacheClient) {
		f.timeProvider = timeProvider
	}
}

// TimeProvider defines an interface that returns the current time. It's used for testing.
type TimeProvider interface {
	Time() time.Time
}

// currentTimeProvider is a TimeProvider that will always return the current time
type currentTimeProvider int

func (currentTimeProvider) Time() time.Time { return time.Now() }

// NewFileCacheClientWrapper wraps the given Client and caches the responses for the given cacheDuration.
func NewFileCacheClientWrapper(client Client, cacheDir string, cacheDuration time.Duration, options ...Option) Client {

	diskStore := storage.NewJSONDiskStorage(cacheDir)

	fcc := &fileCacheClient{
		client:        client,
		cache:         diskStore,
		cacheDuration: cacheDuration,
		timeProvider:  currentTimeProvider(0),
	}

	for _, opt := range options {
		opt(fcc)
	}

	return fcc
}

// fileCacheClient is an API Server client wrapper that caches responses on disk.
type fileCacheClient struct {
	client        Client
	cache         storage.Storage
	cacheDuration time.Duration
	timeProvider  TimeProvider
}

func getType(obj interface{}) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

func cacheKey(obj interface{}, objectName string) string {
	objectType := getType(obj)
	return fmt.Sprintf("%s.%s", objectType, objectName)
}

func (f *fileCacheClient) cacheExpired(cacheTime int64) bool {
	now := f.timeProvider.Time().Unix()
	return cacheTime+int64(f.cacheDuration.Seconds()) < now
}

func (f *fileCacheClient) load(obj interface{}, objectName string) bool {
	cacheTime, err := f.cache.Read(cacheKey(obj, objectName), obj)
	if err != nil {
		return false
	}

	return !f.cacheExpired(cacheTime)
}

func (f *fileCacheClient) store(obj interface{}, objectName string) error {
	return f.cache.Write(cacheKey(obj, objectName), obj)
}

func (f *fileCacheClient) GetNodeInfo(nodeName string) (*NodeInfo, error) {

	n := &NodeInfo{}

	if f.load(n, nodeName) {
		return n, nil
	}

	n, err := f.client.GetNodeInfo(nodeName)
	if err != nil {
		return nil, err
	}

	return n, f.store(n, n.NodeName)
}
