package client

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v2/src/storage"
)

// DiscoveryCacher implements the Discoverer API to read endpoints from a cache storage. It also wraps another
// Discoverer and uses it to discover endpoints when the data is not found in the cache.
// This type is not thread-safe.
type DiscoveryCacher struct {
	DiscoveryCacherConfig

	// Discoverer points to the wrapped Discovered used to resolve endpoints when they are not found in the cache
	Discoverer Discoverer
	Compose    Composer
	Decompose  Decomposer

	// CachedDataPtr must be a pointer to an object where the data will be unmarshalled to
	CachedDataPtr interface{}
	// StorageKey is the key for the Storage Cache
	StorageKey string
}

// DiscoveryCacherConfig defines common properties for discovery cachers.
type DiscoveryCacherConfig struct {
	Storage   storage.Storage
	TTL       time.Duration
	TTLJitter uint
	Logger    *logrus.Logger
}

// Decomposer implementors must convert a HTTPClient into a data structure that can be Stored in the cache.
type Decomposer func(source HTTPClient) (interface{}, error)

// Composer implementors must convert the data from the cached entities to a Client.
type Composer func(source interface{}, cacher *DiscoveryCacher, timeout time.Duration) (HTTPClient, error)

// Discover tries to retrieve a HTTPClient from the cache, and otherwise engage the discovery process from the wrapped
// Discoverer
func (d *DiscoveryCacher) Discover(timeout time.Duration) (HTTPClient, error) {
	creationTimestamp, err := d.Storage.Read(d.StorageKey, d.CachedDataPtr)
	if err != nil {
		d.Logger.Debugf("Cached %q not found. Triggering discovery process", d.StorageKey)

		return d.discoverAndCache(timeout)
	}

	d.Logger.Debugf("Found cached copy of %q stored at %s", d.StorageKey, time.Unix(creationTimestamp, 0))

	// Check cached object TTL
	if Expired(time.Now(), creationTimestamp, d.TTL, d.TTLJitter) {
		d.Logger.Debugf("Cached copy of %q expired. Refreshing", d.StorageKey)

		return d.discoverAndCache(timeout)
	}

	wrappedClient, err := d.Compose(d.CachedDataPtr, d, timeout)
	if err != nil {
		return nil, err
	}

	return d.wrap(wrappedClient, timeout), nil
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
		d.Logger.Warnf("Could not store %q in the cache: %v", d.StorageKey, err)
	}
	return d.wrap(client, timeout), nil
}

// Expired checks, if for a given current time, object creation timestamp and TTL, object should be
// considered as expired (TTL has been exceeded).
//
// If jitter max percentage is not zero, TTL will be either increased or decreased randomly by maximum of selected
// TTL percentage. This allows to distribute cache expiration in time to avoid all caches to expire at the same time
// in multiple clients, e.g. in distributed systems. As an example:
//
// For a TTL of 100 and jitter max percentage of 20, TTL will be within range of 80-120.
//
// If jitter max percentage is 0, TTL remains as given.
func Expired(currentTime time.Time, creationTimestamp int64, ttl time.Duration, jitterMaxPercentage uint) bool {
	rand.Seed(time.Now().UTC().UnixNano())

	// Convert e.g. 20% to 0.2
	jitterPercentage := float64(jitterMaxPercentage) / 100

	// Random number between -1 and 1.
	randomFactor := ((rand.Float64() * 2) - 1)

	// Add extra 1 so we use original TTL +- jitter, otherwise we would get just jitter computed.
	ttlMultiplier := (jitterPercentage * randomFactor) + 1

	// Multiply TTL as float with multiplier, then convert back to duration in seconds.
	ttlWithJitter := time.Duration(ttl.Seconds()*ttlMultiplier) * time.Second

	// As in documentation, time.Now().Sub() is the same as time.Since().
	return currentTime.Sub(time.Unix(creationTimestamp, 0)) > ttlWithJitter
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
			c.cacher.Logger.Debugf("Could not remove %q from the cache: %v", c.cacher.StorageKey, err)
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

// MultiDiscoveryCacher is a wrapper for MultiDiscoverer implementations that can cache the results into some storages.
// It implements the MultiDiscoverer interface.
// This type is not threadsafe.
type MultiDiscoveryCacher struct {
	DiscoveryCacherConfig

	Discoverer MultiDiscoverer
	Compose    MultiComposer
	Decompose  MultiDecomposer

	// CachedDataPtr must be a pointer to an object where the data will be unmarshalled to
	CachedDataPtr interface{}
	// StorageKey is the key for the Storage Cache
	StorageKey string
}

// Discover runs the underlying discovery and caches its result.
// If there is a non-expired discover cache in the storage, it will be loaded.
// If the cache is not present or has expired, it will be written.
// If the cache read fails, the underlying discovery will still run.
func (d *MultiDiscoveryCacher) Discover(timeout time.Duration) ([]HTTPClient, error) {
	creationTimestamp, err := d.Storage.Read(d.StorageKey, d.CachedDataPtr)
	if err == nil {
		d.Logger.Debugf("Found cached copy of %q stored at %s", d.StorageKey, time.Unix(creationTimestamp, 0))
		// Check cached object TTL
		if !Expired(time.Now(), creationTimestamp, d.TTL, d.TTLJitter) {
			clients, err := d.Compose(d.CachedDataPtr, d, timeout)
			if err != nil {
				return nil, errors.Wrap(err, "could not compose cache")
			}
			return clients, nil
		}
		d.Logger.Debugf("Cached copy of %q expired. Refreshing", d.StorageKey)
	} else {
		d.Logger.Debugf("Cached %q not found. Triggering discovery process", d.StorageKey)
	}
	clients, err := d.discoverAndCache(timeout)
	if err != nil {
		return nil, err
	}
	return clients, nil
}

func (d *MultiDiscoveryCacher) discoverAndCache(timeout time.Duration) ([]HTTPClient, error) {
	clients, err := d.Discoverer.Discover(timeout)
	if err != nil {
		return nil, err
	}
	toCache, err := d.Decompose(clients)
	if err == nil {
		err = d.Storage.Write(d.StorageKey, toCache)
	}
	if err != nil {
		d.Logger.Warnf("Could not store %q in the cache: %v", d.StorageKey, err)
	}
	return clients, nil
}

// MultiDecomposer implementors must convert a HTTPClient into a data structure that can be Stored in the cache.
type MultiDecomposer func(sources []HTTPClient) (interface{}, error)

// MultiComposer implementors must convert the cached data to a []HTTPClient.
type MultiComposer func(source interface{}, cacher *MultiDiscoveryCacher, timeout time.Duration) ([]HTTPClient, error)
