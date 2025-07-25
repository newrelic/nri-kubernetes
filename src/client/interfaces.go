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

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
