package client

import (
	"net/http"
	"time"
)

// Discoverer allows discovering the endpoints from different services in the Kubernetes ecosystem.
type Discoverer interface {
	Discover(timeout time.Duration) (HTTPClient, error)
}

// MultiDiscoverer allows for discovery processes that return more
// than a single HTTP client. For example, in one node we might query
// more than one KSM instance, so we need two clients.
type MultiDiscoverer interface {
	Discover(timeout time.Duration) ([]HTTPClient, error)
}

// HTTPClient allows to connect to the discovered Kubernetes services
type HTTPClient interface {
	HTTPGetter
	NodeIPGetter
}

// HTTPGetter is an interface for HTTP client with, which should provide
// scheme, port and hostname for the HTTP call.
type HTTPGetter interface {
	Get(path string) (*http.Response, error)
}

// NodeIPGetter allows getting discovered Node IP.
type NodeIPGetter interface {
	NodeIP() string
}
