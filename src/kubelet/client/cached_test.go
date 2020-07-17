package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
)

const defaultNodeName = "the-node-name"

func TestDiscover_Cache_HTTP(t *testing.T) {
	c := mockedClient()
	onFindNode(c, defaultNodeName, "1.2.3.4", defaultInsecureKubeletPort)

	// And a disk cache storage
	tmpDir, err := ioutil.TempDir("", "test_discover_cached_kubelet")
	assert.NoError(t, err)
	storage := storage.NewJSONDiskStorage(tmpDir)
	// and an Discoverer implementation
	wrappedDiscoverer := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: allOkConnectionChecker,
		logger:      logger,
	}

	// And a Kubelet Discovery Cacher
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, storage, time.Hour, logger)

	// That successfully retrieved the insecure Kubelet URL
	caClient, err := cacher.Discover(timeout)
	kclient := client.WrappedClient(caClient)
	assert.Equal(t, "d34db33f", kclient.(*kubelet).config.BearerToken)

	// When invoking again the discovery process, it should not use the API client
	wrappedDiscoverer.apiClient = failingClientMock()
	caClient, err = cacher.Discover(timeout)
	kclient = client.WrappedClient(caClient)

	// And the returned cached instance should be correctly configured
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:10255", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "http", kclient.(*kubelet).endpoint.Scheme)
	assert.Equal(t, "d34db33f", kclient.(*kubelet).config.BearerToken)
	assert.Equal(t, "d34db33f", kclient.(*kubelet).config.BearerToken)
	assert.Equal(t, defaultNodeName, kclient.(*kubelet).nodeName)
	assert.Nil(t, kclient.(*kubelet).httpClient.Transport)
}

func TestDiscover_Cache_HTTPS_InsecureClient(t *testing.T) {
	c := mockedClient()
	onFindNode(c, defaultNodeName, "1.2.3.4", defaultSecureKubeletPort)

	// And a disk cache storage
	tmpDir, err := ioutil.TempDir("", "test_discover_cached_kubelet")
	assert.NoError(t, err)
	storage := storage.NewJSONDiskStorage(tmpDir)
	// and an Discoverer implementation
	wrappedDiscoverer := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: allOkConnectionChecker,
		logger:      logger,
	}

	// And a Kubelet Discovery Cacher
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, storage, time.Hour, logger)

	// That successfully retrieved the secure Kubelet URL
	caClient, err := cacher.Discover(timeout)

	// When invoking again the discovery process, it should not use the API client
	wrappedDiscoverer.apiClient = failingClientMock()
	caClient, err = cacher.Discover(timeout)

	// The call works correctly
	assert.NoError(t, err)
	// And the cached host:port of the Kubelet is returned
	kclient := client.WrappedClient(caClient)
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:10250", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "https", kclient.(*kubelet).endpoint.Scheme)
	assert.Equal(t, "d34db33f", kclient.(*kubelet).config.BearerToken)
	assert.Equal(t, defaultNodeName, kclient.(*kubelet).nodeName)
	assert.True(t, kclient.(*kubelet).httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
}

func TestDiscover_Cache_HTTPS_SecureClient(t *testing.T) {
	c := mockedClient()
	// In a node whose Kubelet endpoint has not an standard port
	onFindNode(c, defaultNodeName, "1.2.3.4", 55332)

	// And a disk cache storage
	tmpDir, err := ioutil.TempDir("", "test_discover_cached_kubelet")
	assert.NoError(t, err)
	storage := storage.NewJSONDiskStorage(tmpDir)
	// and an Discoverer implementation
	wrappedDiscoverer := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: onlyAPIConnectionChecker,
		logger:      logger,
	}

	// And a Kubelet Discovery Cacher
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, storage, time.Hour, logger)

	// That successfully retrieved the secure Kubelet API URL
	caClient, err := cacher.Discover(timeout)

	// When invoking again the discovery process, it should not use the API client
	wrappedDiscoverer.apiClient = failingClientMock()
	caClient, err = cacher.Discover(timeout)

	// The call works correctly
	assert.NoError(t, err)
	// And the cached host:port of the Kubelet is returned
	kclient := client.WrappedClient(caClient)
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, fakeDiscoveredAPIHost, kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "https", kclient.(*kubelet).endpoint.Scheme)
	assert.Equal(t, "/api/v1/nodes/the-node-name/proxy/", kclient.(*kubelet).endpoint.Path)
	assert.Equal(t, "d34db33f", kclient.(*kubelet).config.BearerToken)
}

func TestDiscover_Cache_DiscoveryError(t *testing.T) {
	c := mockedClient()

	// That doesn't find node so it isn't going to be able to find the kubelet host IP
	c.On("FindNode", defaultNodeName).Return(&v1.Node{}, fmt.Errorf("Node not found"))

	// And a disk cache storage
	tmpDir, err := ioutil.TempDir("", "test_discover_cached_kubelet")
	assert.NoError(t, err)
	storage := storage.NewJSONDiskStorage(tmpDir)
	// and an Discoverer implementation
	wrappedDiscoverer := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: onlyAPIConnectionChecker,
		logger:      logger,
	}

	// And a Kubelet Discovery Cacher without any cached data
	cacher := NewDiscoveryCacher(&wrappedDiscoverer, storage, time.Hour, logger)

	// When retrieving the Kubelet client
	_, err = cacher.Discover(timeout)
	// The system returns an error
	assert.Error(t, err)
}
