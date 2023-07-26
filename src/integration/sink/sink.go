package sink

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const (
	// DefaultAgentForwarderhost holds the default endpoint of the agent forwarder.
	DefaultAgentForwarderhost = "localhost"
	DefaultAgentForwarderPath = "/v1/data"
)

// Doer is the interface that HTTPSink client should satisfy.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPSink holds the configuration of the HTTP sink used by the integration.
type HTTPSink struct {
	url    string
	client Doer
}

// HTTPSinkOptions holds the configuration of the HTTP sink used by the integration.
type HTTPSinkOptions struct {
	URL    string
	Client Doer
}

// New initialize HTTPSink struct.
func New(options HTTPSinkOptions) (*HTTPSink, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if options.URL == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	return &HTTPSink{
		url:    options.URL,
		client: options.Client,
	}, nil
}

// Write is the function signature needed by the infrastructure SDK package.
func (h HTTPSink) Write(p []byte) (n int, err error) {
	request, err := http.NewRequest("POST", h.url, bytes.NewBuffer(p))
	if err != nil {
		return 0, fmt.Errorf("preparing request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("performing HTTP request: %w", err)
	}

	defer cleanBody(resp)

	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("unexpected status code: %d, expected: %d", resp.StatusCode, http.StatusNoContent)
	}

	return len(p), nil
}

func cleanBody(resp *http.Response) {
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		log.Error("reading body", err)
	}

	if err := resp.Body.Close(); err != nil {
		log.Error("closing body", err)
	}
}
