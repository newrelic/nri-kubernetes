package client

import (
	"net/http"
	"time"

	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
)

// DiscoveryCacher implements the Discoverer API to read endpoints from a cache storage. It also wraps another
// Discoverer and uses it to discover endpoints when the data is not found in the cache.
// This type is not thread-safe.
type DiscoveryCacher struct {
	// CachedDataPtr must be a pointer to an object where the data will be unmarshalled to
	CachedDataPtr interface{}
	// StorageKey is the key for the Storage Cache
	StorageKey string
	// Discoverer points to the wrapped Discovered used to resolve endpoints when they are not found in the cache
	Discoverer Discoverer
	// Storage for cached data
	Storage   storage.Storage
	TTL       time.Duration
	Logger    *logrus.Logger
	Compose   Composer
	Decompose Decomposer
}

// Decomposer implementors must convert a HTTPClient into a data structure that can be Stored in the cache.
type Decomposer func(source HTTPClient) (interface{}, error)

// Composer implementors must convert the data from the cached entities to a Client.
type Composer func(source interface{}, cacher *DiscoveryCacher, timeout time.Duration) (HTTPClient, error)

// Discover tries to retrieve a HTTPClient from the cache, and otherwise engage the discovery process from the wrapped
// Discoverer
func (d *DiscoveryCacher) Discover(timeout time.Duration) (HTTPClient, error) {
	ts, err := d.Storage.Read(d.StorageKey, d.CachedDataPtr)
	if err == nil {
		d.Logger.Debugf("Found cached copy of %q stored at %s", d.StorageKey, time.Unix(ts, 0))
		// Check cached object TTL
		if time.Now().Unix() < ts+int64(d.TTL.Seconds()) {
			wrappedClient, err := d.Compose(d.CachedDataPtr, d, timeout)
			if err != nil {
				return nil, err
			}
			return d.wrap(wrappedClient, timeout), nil
		}
		d.Logger.Debugf("Cached copy of %q expired. Refreshing", d.StorageKey)
	} else {
		d.Logger.Debugf("Cached %q not found. Triggering discovery process", d.StorageKey)
	}
	client, err := d.discoverAndCache(timeout)
	if err != nil {
		return nil, err
	}
	return d.wrap(client, timeout), nil
}

func (d *DiscoveryCacher) discoverAndCache(timeout time.Duration) (HTTPClient, error) {
	client, err := d.Discoverer.Discover(timeout)
	if err != nil {
		return nil, err
	}
	// and store the discovered data into the cache
	toCache, err := d.Decompose(client)
	if err == nil {
		err = d.Storage.Write(d.StorageKey, toCache)
	}
	if err != nil {
		d.Logger.WithError(err).Warnf("while storing %q in the cache", d.StorageKey)
	}
	return client, nil
}

func (d *DiscoveryCacher) wrap(client HTTPClient, timeout time.Duration) *cacheAwareClient {
	return &cacheAwareClient{
		client:  client,
		cacher:  d,
		timeout: timeout,
	}
}

// cacheAwareClient wraps the cached client and if it fails because it has outdated data, retriggers the
type cacheAwareClient struct {
	client  HTTPClient
	cacher  *DiscoveryCacher
	timeout time.Duration
}

func (c *cacheAwareClient) Do(method, path string) (*http.Response, error) {
	response, err := c.client.Do(method, path)
	if err == nil {
		return response, nil
	}
	// If the Do invocation returns error, retriggers the discovery process.
	// A response with an HTTP status error is considered successful from the cache side (it discovered correctly
	// the server that has returned the error)
	newClient, err := c.cacher.discoverAndCache(c.timeout)
	if err != nil {
		// If the client can't be rediscovered, it anyway invalidates the cache
		if err := c.cacher.Storage.Delete(c.cacher.StorageKey); err != nil {
			c.cacher.Logger.WithError(err).Debugf("while trying to remove %q from the cache", c.cacher.StorageKey)
		}
		return nil, err
	}
	c.client = newClient
	return c.client.Do(method, path)
}

// this implementation doesn't guarantee the returned NodeIP is valid in the moment of the function invocation.
func (c *cacheAwareClient) NodeIP() string {
	return c.client.NodeIP()
}

// WrappedClient is only aimed for testing. It allows extracting the wrapped client of a given cacheAwareClient.
func WrappedClient(caClient HTTPClient) HTTPClient {
	return caClient.(*cacheAwareClient).client
}
