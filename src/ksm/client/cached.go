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
