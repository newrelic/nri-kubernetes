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
	Do(method, path string) (*http.Response, error)
	NodeIP() string
}
