package client

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logger = logrus.StandardLogger()
var timeout = time.Second
var storageKey = "mock-discovery-client"

// Fake storage that backs data in-memory
type fakeStorage struct {
	values map[string]interface{}
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		values: map[string]interface{}{},
	}
}

func (f *fakeStorage) Write(key string, value interface{}) error {
	f.values[key] = value
	return nil
}

func (f *fakeStorage) Read(key string, valuePtr interface{}) (int64, error) {
	valPtr, ok := f.values[key].(*MockedKubernetes)
	if !ok {
		return 0, fmt.Errorf("key not found: %s", key)
	}
	*(valuePtr.(*MockedKubernetes)) = *valPtr //nolint: govet
	return int64(1234567), nil
}

func (f *fakeStorage) Delete(key string) error {
	_, ok := f.values[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	delete(f.values, key)
	return nil
}

func discoveryCacher(client HTTPClient, discoverer Discoverer, st storage.Storage) *DiscoveryCacher {
	return &DiscoveryCacher{
		StorageKey: storageKey,
		Discoverer: discoverer,
		Storage:    st,
		Logger:     logger,
		// Since we use just memory, Compose and Decompose are just identity functions
		Compose: func(source interface{}, _ *DiscoveryCacher, _ time.Duration) (HTTPClient, error) {
			return source.(HTTPClient), nil
		},
		Decompose: func(source HTTPClient) (interface{}, error) {
			return client, nil
		},
	}
}

func TestCacheAwareClient_CachedClientWorks(t *testing.T) {
	// Setup storage
	store := newFakeStorage()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("NodeIP").Return("1.2.3.4")
	wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).
		Once() // Expectation: the discovery process will be invoked only once

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer, store)

	// That discovers a client
	client, err := cacher.Discover(timeout)
	assert.NoError(t, err)

	// When the client works as expected
	resp, err := client.Do("GET", "/api/path")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// The Discovery process has been triggered once
	discoverer.AssertExpectations(t)
}

func TestCacheAwareClient_CachedClientDoesNotWork(t *testing.T) {
	// Setup storage
	store := newFakeStorage()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("NodeIP").Return("1.2.3.4")
	// After the error on the first call, the second call returns a correct value
	wrappedClient.On("Do", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("patapum")).Once()
	wrappedClient.On("Do", mock.Anything, mock.Anything).
		Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).
		Twice() // Expectation: the discovery process will be invoked twice

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer, store)

	// That discovers a client
	client, err := cacher.Discover(timeout)
	assert.NoError(t, err)

	// When the cached client does not work (see discovered client mock setup)
	resp, err := client.Do("GET", "/api/path")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// The Discovery process has been triggered again
	discoverer.AssertExpectations(t)
}

func TestCacheAwareClient_RediscoveryDoesntWork(t *testing.T) {
	// Setup storage
	store := newFakeStorage()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("NodeIP").Return("1.2.3.4")
	wrappedClient.On("Do", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("patapum"))

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	// Expectations: the discovery process will work but the "re-discovery" in case of failure, not
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).Once()
	discoverer.On("Discover", mock.Anything).Return((*MockDiscoveredHTTPClient)(nil), fmt.Errorf("discovery failed"))

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer, store)

	// That discovers a client
	client, err := cacher.Discover(timeout)
	assert.NoError(t, err)

	// When the cached client does not work and neither the re-discovery do
	resp, err := client.Do("GET", "/api/path")
	assert.Equal(t, "discovery failed", err.Error())
	assert.Nil(t, resp)

	// The Discovery process has not been triggered more times
	discoverer.AssertExpectations(t)

	// And the cache has been invalidated
	_, err = store.Read(storageKey, &struct{}{})
	assert.Error(t, err)
}
