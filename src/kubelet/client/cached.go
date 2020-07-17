package client

import (
	"net/http"
	"net/url"
	"time"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/storage"
	"github.com/sirupsen/logrus"
)

const cachedKey = "kubelet-client"

// cache holds the data to be cached for a Kubelet client.
// Its fields must be public to make them visible for the JSON Marshaller.
type cache struct {
	Endpoint    url.URL
	NodeIP      string
	NodeName    string
	HTTPType    int
	BearerToken string
}

// compose implements the ClientComposer function signature
func compose(source interface{}, cacher *client.DiscoveryCacher, timeout time.Duration) (client.HTTPClient, error) {
	cached := source.(*cache)
	kd := cacher.Discoverer.(*discoverer)
	var c *http.Client
	switch cached.HTTPType {
	case httpInsecure:
		c = client.InsecureHTTPClient(timeout)
	case httpSecure:
		api, err := kd.connectionAPIHTTPS(cached.NodeName, timeout)
		if err != nil {
			return nil, err
		}
		c = api.client
	default:
		c = client.BasicHTTPClient(timeout)
	}
	return newKubelet(cached.NodeIP, cached.NodeName, cached.Endpoint, cached.BearerToken, c, cached.HTTPType, kd.logger), nil
}

// decompose implements the ClientDecomposer function signature
func decompose(source client.HTTPClient) (interface{}, error) {
	kc := source.(*kubelet)
	return &cache{
		Endpoint:    kc.endpoint,
		NodeIP:      kc.nodeIP,
		NodeName:    kc.nodeName,
		HTTPType:    kc.httpType,
		BearerToken: kc.config.BearerToken,
	}, nil
}

// NewDiscoveryCacher creates a new DiscoveryCacher that wraps a discoverer and caches the data into the
// specified storage
func NewDiscoveryCacher(discoverer client.Discoverer, storage storage.Storage, ttl time.Duration, logger *logrus.Logger) *client.DiscoveryCacher {
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
