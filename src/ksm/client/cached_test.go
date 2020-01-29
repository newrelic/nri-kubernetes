package client

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDiscover_Cache(t *testing.T) {
	// Setup cache directory
	tmpDir, err := ioutil.TempDir("", "test_discover")
	assert.NoError(t, err)

	// Setup Kubernetes API client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// Setup storage
	store := storage.NewJSONDiskStorage(tmpDir)

	// Given a KSM discoverer
	wrappedDiscoverer := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}
	// That is wrapped into a Cached Discoverer
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, &store, time.Hour, logger)

	// And previously has discovered the HTTP Client
	caClient, err := cacher.Discover(timeout)

	// When the discovery process is invoked again
	wrappedDiscoverer.lookupSRV = failingLookupSRV
	caClient, err = cacher.Discover(timeout)
	assert.NoError(t, err)

	// The cached value has been retrieved, instead of triggered the discovery
	// (otherwise it would have failed when invoking the `failedLookupSRV` and the unconfigured mock
	assert.NoError(t, err)
	ksmClient := client.WrappedClient(caClient)
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	assert.Equal(t, "6.7.8.9", caClient.NodeIP())
}

func TestDiscover_Cache_BothFail(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test_discover")
	assert.NoError(t, err)

	// Given a client that is unable to discover the endpoint
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{}}, fmt.Errorf("error invoking Kubernetes API"))
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{Items: []v1.Service{}}, fmt.Errorf("error invoking Kubernetes API"))

	// And a cache that does not store any cached copy
	store := storage.NewJSONDiskStorage(tmpDir)

	// And a Cached KSM discoverer
	cacher := NewDiscoveryCacher(
		&discoverer{
			lookupSRV: fakeLookupSRV,
			apiClient: c,
			logger:    logger,
		}, &store, time.Hour, logger)

	// The Discover invocation should return error
	_, err = cacher.Discover(timeout)
	assert.Error(t, err)
}

func TestDiscover_LoadCacheFail(t *testing.T) {
	// Setup cache directory
	tmpDir, err := ioutil.TempDir("", "test_discover")
	assert.NoError(t, err)

	// Setup Kubernetes API client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// Setup storage
	store := storage.NewJSONDiskStorage(tmpDir)

	// Given a KSM discoverer
	wrappedDiscoverer := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}
	// That is wrapped into a Cached Discoverer
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, &store, time.Hour, logger)

	// And previously has discovered the KSM endpoint
	caClient, err := cacher.Discover(timeout)

	// But the cache stored data is corrupted
	assert.Nil(t, store.Write(cachedKey, "corrupt-data"))

	// When the discovery process is invoked again
	caClient, err = cacher.Discover(timeout)

	// The discovery process has been triggered again
	assert.NoError(t, err)
	ksmClient := client.WrappedClient(caClient)
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	assert.Equal(t, "6.7.8.9", caClient.NodeIP())
}

func TestDiscover_CacheTTLExpiry(t *testing.T) {
	// Setup cache directory
	tmpDir, err := ioutil.TempDir("", "test_discover")
	assert.NoError(t, err)

	// Setup Kubernetes API client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// Setup storage
	store := storage.NewJSONDiskStorage(tmpDir)

	// Given an outdated version of a stored object
	tu, _ := url.Parse("http://1.2.3.4")
	outdatedData := cache{
		Endpoint: *tu,
		NodeIP:   "1.2.3.4",
	}
	assert.NoError(t, store.Write(cachedKey, &outdatedData))

	// And a KSM discoverer
	wrappedDiscoverer := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}
	// That is wrapped into a Cached Discoverer
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, &store, -time.Second, logger)

	// When the discovery process tries to get the data from the cache
	caClient, err := cacher.Discover(timeout)

	// The outdated version of the object has been invalidated
	// and the discovery process has been triggered again
	assert.NoError(t, err)
	ksmClient := client.WrappedClient(caClient)
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	assert.Equal(t, "6.7.8.9", caClient.NodeIP())

	// And the new object has been cached back
	wrappedDiscoverer.lookupSRV = failingLookupSRV
	cacher.(*client.DiscoveryCacher).TTL = time.Hour
	caClient, err = cacher.Discover(timeout)
	// (otherwise it would have failed when invoking the `failedLookupSRV` and the unconfigured mock
	assert.NoError(t, err)
	ksmClient = client.WrappedClient(caClient)
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	assert.Equal(t, "6.7.8.9", caClient.NodeIP())
}

func TestMultiDiscover_Cache(t *testing.T) {
	// Temporary directory to store the cache.
	tmpDir, err := ioutil.TempDir("", "test_multi_discover")
	assert.NoError(t, err)

	// Mock out the access to the Kubernetes API when looking up pods by label.
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.4"}},
			{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.5"}},
			{Status: v1.PodStatus{HostIP: "6.7.8.10", PodIP: "1.2.3.6"}},
		}}, nil)

	// Cache storage.
	cacheStore := storage.NewJSONDiskStorage(tmpDir)

	// Creates a distribute discovery with cache.
	wrappedDiscoverer := distributedPodLabelDiscoverer{
		ownNodeIP: "6.7.8.9",
		k8sClient: c,
		logger:    logger,
	}
	cacher := NewDistributedDiscoveryCacher(&wrappedDiscoverer, &cacheStore, time.Hour, logger)

	clients, err := cacher.Discover(timeout)
	assert.Len(t, clients, 2)
	assert.NoError(t, err)

	cachedClients, err := cacher.Discover(timeout)
	assert.Len(t, clients, 2)
	assert.NoError(t, err)

	assert.ElementsMatch(t, cachedClients, clients)
}

func TestMultiDiscover_CacheWithError(t *testing.T) {
	// Temporary directory to store the cache.
	tmpDir, err := ioutil.TempDir("", "test_multi_discover")
	assert.NoError(t, err)

	// Mock out the access to the Kubernetes API when looking up pods by label.
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{}, fmt.Errorf("error invoking Kubernetes API"))

	// Cache storage.
	cacheStore := storage.NewJSONDiskStorage(tmpDir)

	// Creates a distribute discovery with cache.
	wrappedDiscoverer := distributedPodLabelDiscoverer{
		ownNodeIP: "6.7.8.9",
		k8sClient: c,
		logger:    logger,
	}
	cacher := NewDistributedDiscoveryCacher(&wrappedDiscoverer, &cacheStore, time.Hour, logger)

	clients, err := cacher.Discover(timeout)
	assert.Error(t, err)
	assert.Nil(t, clients)

	cachedClients, err := cacher.Discover(timeout)
	assert.Error(t, err)
	assert.Nil(t, cachedClients)
}

func TestMultiDiscover_CacheCorrupted(t *testing.T) {
	// Temporary directory to store the cache.
	tmpDir, err := ioutil.TempDir("", "test_multi_discover")
	assert.NoError(t, err)

	// Mock out the access to the Kubernetes API when looking up pods by label.
	pod1 := v1.Pod{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.4"}}
	pod2 := v1.Pod{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.5"}}
	pod3 := v1.Pod{Status: v1.PodStatus{HostIP: "6.7.8.10", PodIP: "1.2.3.6"}}
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			pod1, pod2, pod3,
		}}, nil)

	// Cache storage.
	cacheStore := storage.NewJSONDiskStorage(tmpDir)

	// Creates a distribute discovery with cache.
	wrappedDiscoverer := distributedPodLabelDiscoverer{
		ownNodeIP: "6.7.8.9",
		k8sClient: c,
		logger:    logger,
	}
	cacher := NewDistributedDiscoveryCacher(&wrappedDiscoverer, &cacheStore, time.Hour, logger)

	assert.Nil(t, cacheStore.Write(cachedKey, "corrupt-data"))
	clients, err := cacher.Discover(timeout)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(clients))
}

func TestMultiDiscover_CacheTTL(t *testing.T) {
	// Temporary directory to store the cache.
	tmpDir, err := ioutil.TempDir("", "test_multi_discover")
	// Cache storage.
	cacheStore := storage.NewJSONDiskStorage(tmpDir)
	assert.NoError(t, err)

	// Mock out the access to the Kubernetes API when looking up pods by label.
	c := new(client.MockedKubernetes)
	outdatedURL, _ := url.Parse("http://1.2.3.4")
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: outdatedURL.Host}},
		}}, nil).Once()

	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.4"}},
			{Status: v1.PodStatus{HostIP: "6.7.8.9", PodIP: "1.2.3.5"}},
			{Status: v1.PodStatus{HostIP: "6.7.8.10", PodIP: "1.2.3.6"}},
		}}, nil).Once()

	// Creates a distribute discovery with cache.
	wrappedDiscoverer := distributedPodLabelDiscoverer{
		ownNodeIP: "6.7.8.9",
		k8sClient: c,
		logger:    logger,
	}
	cacher := NewDistributedDiscoveryCacher(&wrappedDiscoverer, &cacheStore, -time.Hour, logger)

	clients, err := cacher.Discover(timeout)
	assert.Len(t, clients, 1)
	assert.NoError(t, err)

	cachedClients, err := cacher.Discover(timeout)
	assert.Len(t, cachedClients, 2)
	assert.NoError(t, err)

	// We should not see the outdated host's URL anymore.
	for _, cachedClient := range cachedClients {
		assert.NotEqual(t, cachedClient.(*ksm).endpoint, outdatedURL.Host)
	}
}
