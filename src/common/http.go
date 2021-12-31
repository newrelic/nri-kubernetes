package common

import (
	"fmt"
	"net/http"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// HTTPClient implements convenience functions for component clients to use.
type HTTPClient struct {
	Doer HTTPDoer
}

func NewHTTP(doer HTTPDoer) HTTPClient {
	return HTTPClient{
		Doer: doer,
	}
}

func (h HTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request object: %w", err)
	}

	response, err := h.Doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making http request to %q: %w", url, err)
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("request to %q returned non-200 status code: %d", url, response.StatusCode)
	}

	return response, nil
}
