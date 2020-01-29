package client

import (
	"net/http"
	"net/url"
	"time"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
)

const cachedKey = "ksm-client"

// cache holds the data to be cached for a KSM client.
// Its fields must be public to make them visible for the JSON Marshaller.
type cache struct {
	Endpoint url.URL
	NodeIP   string
}

// compose implements the ClientComposer function signature
func compose(source interface{}, cacher *client.DiscoveryCacher, timeout time.Duration) (client.HTTPClient, error) {
	cached := source.(*cache)
	return &ksm{
		nodeIP:   cached.NodeIP,
		endpoint: cached.Endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: cacher.Logger,
	}, nil
}

// decompose implements the ClientDecomposer function signature
func decompose(source client.HTTPClient) (interface{}, error) {
	ksm := source.(*ksm)
	return &cache{
		Endpoint: ksm.endpoint,
		NodeIP:   ksm.nodeIP,
	}, nil
}

// NewDiscoveryCacher creates a new DiscoveryCacher that wraps a discoverer and caches the data into the
// specified storage
func NewDiscoveryCacher(discoverer client.Discoverer, storage storage.Storage, ttl time.Duration, logger *logrus.Logger) client.Discoverer {
	return &client.DiscoveryCacher{
		CachedDataPtr: &cache{},
		StorageKey:    cachedKey,
		Discoverer:    discoverer,
		Storage:       storage,
		TTL:           ttl,
		Logger:        logger,
		Compose:       compose,
		Decompose:     decompose,
	}
}

// multiCache holds the data to be cached for many KSM clients.
// Its fields must be public to make them visible for the JSON Marshaller.
type multiCache struct {
	Endpoints []url.URL
	NodeIP    string
}

// multiCompose implements the MultiComposer function signature
func multiCompose(source interface{}, cacher *client.MultiDiscoveryCacher, timeout time.Duration) ([]client.HTTPClient, error) {
	cached := source.(*multiCache)
	var ksmClients []client.HTTPClient
	for _, endpoint := range cached.Endpoints {
		ksmClient := &ksm{
			nodeIP:   cached.NodeIP,
			endpoint: endpoint,
			httpClient: &http.Client{
				Timeout: timeout,
			},
			logger: cacher.Logger,
		}
		ksmClients = append(ksmClients, ksmClient)
	}
	return ksmClients, nil
}

// multiDecompose implements the MultiDecomposer function signature
func multiDecompose(sources []client.HTTPClient) (interface{}, error) {
	ksmCache := &multiCache{}
	for _, source := range sources {
		ksm := source.(*ksm)
		ksmCache.Endpoints = append(ksmCache.Endpoints, ksm.endpoint)
		if ksmCache.NodeIP == "" {
			ksmCache.NodeIP = ksm.nodeIP
		}
	}
	return ksmCache, nil
}

// NewDistributedDiscoveryCacher initializes a client.MultiDiscoveryCacher with the given parameters.
// This should be the only way to create instances of client.MultiDiscoveryCacher, as it guarantees the cached data
// pointer is initialized.
func NewDistributedDiscoveryCacher(innerDiscoverer client.MultiDiscoverer, storage storage.Storage, ttl time.Duration, logger *logrus.Logger) client.MultiDiscoverer {
	return &client.MultiDiscoveryCacher{
		Discoverer:    innerDiscoverer,
		CachedDataPtr: &multiCache{},
		StorageKey:    cachedKey,
		Storage:       storage,
		TTL:           ttl,
		Logger:        logger,
		Compose:       multiCompose,
		Decompose:     multiDecompose,
	}
}
