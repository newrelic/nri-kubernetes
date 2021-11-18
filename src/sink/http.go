package sink

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
)

const (
	// DefaultCtxTimeout is the default IO timeout for the context of the client.
	DefaultCtxTimeout = 15 * time.Second
	// DefaultRequestTimeout is the default IO timeout for each request.
	DefaultRequestTimeout = 15 * time.Second
	// DefaultAgentForwarderhost holds the default endpoint of the agent forwarder.
	DefaultAgentForwarderhost = "localhost"
	DefaultAgentForwarderPath = "/v1/data"
)

// httpSink holds the configuration of the HTTP sink used by the integration.
type httpSink struct {
	url        string
	client     Doer
	ctxTimeout time.Duration
	ctx        context.Context
}

// HTTPSinkOptions holds the configuration of the HTTP sink used by the integration.
type HTTPSinkOptions struct {
	URL        string
	Client     Doer
	CtxTimeout time.Duration
	Ctx        context.Context
}

// Doer is the interface that httpSink client should satisfy.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

//NewHTTPSink initialize httpSink struct.
func NewHTTPSink(options HTTPSinkOptions) (io.Writer, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if options.URL == "" {
		return nil, fmt.Errorf("url cannot be empty")
	}

	if options.CtxTimeout == 0 {
		return nil, fmt.Errorf("contextTimeout cannot be zero")
	}

	if options.Ctx == nil {
		return nil, fmt.Errorf("ctx cannot be nil")
	}

	return &httpSink{
		url:        options.URL,
		client:     options.Client,
		ctxTimeout: options.CtxTimeout,
		ctx:        options.Ctx,
	}, nil
}

// Write is the function signature needed by the infrastructure SDK package.
func (h httpSink) Write(p []byte) (n int, err error) {
	ctx, cancel := context.WithTimeout(h.ctx, h.ctxTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "POST", h.url, bytes.NewBuffer(p))
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
