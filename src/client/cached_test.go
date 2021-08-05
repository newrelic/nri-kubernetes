package client

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

const (
	timeout = time.Second
)

func TestCacheAwareClient_CachedClientWorks(t *testing.T) {
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).
		Once() // Expectation: the discovery process will be invoked only once

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer)

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
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
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
	cacher := discoveryCacher(wrappedClient, discoverer)

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
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("Do", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("patapum"))

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	// Expectations: the discovery process will work but the "re-discovery" in case of failure, not
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).Once()
	discoverer.On("Discover", mock.Anything).Return((*MockDiscoveredHTTPClient)(nil), fmt.Errorf("discovery failed"))

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer)

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
	_, err = cacher.Storage.Read(cacher.StorageKey, &struct{}{})
	assert.Error(t, err)
}

func Test_CacheAwareClient_when_cache_TTL_is_not_reached(t *testing.T) {
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).
		Once() // Expectation: the discovery process will be invoked only once

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer)
	cacher.TTL = time.Hour

	// That discovers a client
	_, err := cacher.Discover(timeout)
	require.NoError(t, err, "running discovery")

	client, err := cacher.Discover(timeout)
	require.NoError(t, err, "running discovery again")

	t.Run("returns_functional_cached_HTTP_client", func(t *testing.T) {
		resp, err := client.Do("GET", "/api/path")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("does_not_perform_the_discovery", func(t *testing.T) {
		// The Discovery process has been triggered once
		discoverer.AssertExpectations(t)
	})
}

func Test_CacheAwareClient_perform_the_discovery_again_when_cache_TTL_is_reached(t *testing.T) {
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil).
		Times(2) // Expectation: the discovery process will be invoked twice.

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer)
	cacher.TTL = time.Second

	// That discovers a client
	_, err := cacher.Discover(timeout)
	require.NoError(t, err, "running discovery")

	time.Sleep(2 * cacher.TTL) // Sleep twice the TTL to make sure we don't hit cache.

	_, err = cacher.Discover(timeout)
	require.NoError(t, err, "running discovery again")

	// The Discovery process has been triggered once.
	discoverer.AssertExpectations(t)
}

func Test_CacheAwareClient_returns_error_when(t *testing.T) {
	t.Parallel()

	t.Run("discovery_fails", func(t *testing.T) {
		t.Parallel()

		// Setup wrapped discoverer
		discoverer := new(MockDiscoverer)
		discoverer.On("Discover", mock.Anything).Return(nil, fmt.Errorf("error"))

		// Given a DiscoveryCacher
		cacher := discoveryCacher(nil, discoverer)

		// That discovers a client
		_, err := cacher.Discover(timeout)
		require.Error(t, err)
	})

	t.Run("cache_composition_fails", func(t *testing.T) {
		t.Parallel()

		// Setup discovered client
		wrappedClient := new(MockDiscoveredHTTPClient)
		wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

		// Setup wrapped discoverer
		discoverer := new(MockDiscoverer)
		discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil)

		// Given a DiscoveryCacher
		cacher := discoveryCacher(wrappedClient, discoverer)
		cacher.TTL = time.Hour
		cacher.Compose = func(_ interface{}, _ *DiscoveryCacher, _ time.Duration) (HTTPClient, error) {
			return nil, fmt.Errorf("error")
		}

		// That discovers a client
		_, err := cacher.Discover(timeout)
		require.NoError(t, err)

		_, err = cacher.Discover(timeout)
		require.Error(t, err)
	})
}

func Test_CacheAwareClient_ignores_cache_decomposition_errors(t *testing.T) {
	t.Parallel()

	// Setup discovered client
	wrappedClient := new(MockDiscoveredHTTPClient)
	wrappedClient.On("Do", mock.Anything, mock.Anything).Return(&http.Response{StatusCode: 200}, nil)

	// Setup wrapped discoverer
	discoverer := new(MockDiscoverer)
	discoverer.On("Discover", mock.Anything).Return(wrappedClient, nil)

	// Given a DiscoveryCacher
	cacher := discoveryCacher(wrappedClient, discoverer)
	cacher.TTL = time.Hour
	cacher.Decompose = func(source HTTPClient) (interface{}, error) {
		return nil, fmt.Errorf("error")
	}

	// That discovers a client
	_, err := cacher.Discover(timeout)
	require.NoError(t, err)
}

func discoveryCacher(client HTTPClient, discoverer Discoverer) *DiscoveryCacher {
	return &DiscoveryCacher{
		CachedDataPtr: &MockDiscoveredHTTPClient{},
		StorageKey:    "mock-discovery-client",
		Discoverer:    discoverer,
		Storage:       &storage.MemoryStorage{},
		Logger:        logrus.StandardLogger(),
		// Since we use just memory, Compose and Decompose are just identity functions
		Decompose: func(source HTTPClient) (interface{}, error) {
			return client, nil
		},
		Compose: func(source interface{}, _ *DiscoveryCacher, _ time.Duration) (HTTPClient, error) {
			return source.(HTTPClient), nil
		},
	}
}
