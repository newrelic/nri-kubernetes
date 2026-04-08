package client

import (
	"net/http"
	"net/url"
)

// HTTPGetter is an interface for HTTP client with, which should provide
// scheme, port and hostname for the HTTP call.
type HTTPGetter interface {
	Get(path string) (*http.Response, error)
	GetURI(uri url.URL) (*http.Response, error)
}

// HTTPGetterWithAccept extends HTTPGetter with support for Accept header content negotiation.
type HTTPGetterWithAccept interface {
	HTTPGetter
	// GetWithAccept sends a GET request with the specified Accept header for content negotiation.
	// The accept parameter should be a valid MIME type (e.g., "application/json", "text/plain").
	GetWithAccept(path string, accept string) (*http.Response, error)
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
