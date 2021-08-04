package client

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

func discoveryCacher(client HTTPClient, discoverer Discoverer) *DiscoveryCacher {
	return &DiscoveryCacher{
		StorageKey: "mock-discovery-client",
		Discoverer: discoverer,
		Storage:    &storage.MemoryStorage{},
		Logger:     logrus.StandardLogger(),
		// Since we use just memory, Compose and Decompose are just identity functions
		Decompose: func(source HTTPClient) (interface{}, error) {
			return client, nil
		},
	}
}
