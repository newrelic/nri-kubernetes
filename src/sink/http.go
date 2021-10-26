package sink

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"
)

const (
	// DefaultTimeout is the default IO timeout for the client.
	DefaultTimeout = 15 * time.Second
	// DefaultAgentForwarderEndpoint holds the default endpoint of the agent forwarder.
	DefaultAgentForwarderEndpoint = "http://localhost:8001/v1/data"
)

//HTTPSink holds the configuration of the HTTP sink used by the integration.
type HTTPSink struct {
	url     string
	client  doer
	timeout time.Duration
}

type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

//NewHTTPSink initialize httpSink struct.
func NewHTTPSink(client doer, url string, contextTimeout time.Duration) (*HTTPSink, error) {
	if client == nil {
		return nil, fmt.Errorf("client of httpSink cannot be nil")
	}

	if url == "" {
		return nil, fmt.Errorf("url of httpSink cannot be empty")
	}

	if contextTimeout == 0 {
		return nil, fmt.Errorf("contextTimeout cannot be zero")
	}

	return &HTTPSink{
		url:     url,
		client:  client,
		timeout: contextTimeout,
	}, nil
}

// Write is the function signature needed by the infrastructure SDK package.
func (h HTTPSink) Write(p []byte) (n int, err error) {
	//Pester gives the possibility to set-up a per-request timeout, that can confusing in this use-case
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "POST", h.url, bytes.NewBuffer(p))
	if err != nil {
		return 0, fmt.Errorf("unable to prepare request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("HTTP transport error: %w", err)
	}

	defer cleanBody(resp)

	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("unexpected statuscode: %s, expected: 204 No Content", resp.Status)
	}

	return len(p), nil
}

//DefaultPesterClient return a defaultPesterClient to be used with httpSink
func DefaultPesterClient() *pester.Client {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 5
	c.LogHook = func(e pester.ErrEntry) {
		log.NewStdErr(false)
	}

	return c
}

func cleanBody(resp *http.Response) {
	_, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		log.Error("reading body", err)
	}

	err = resp.Body.Close()
	if err != nil {
		log.Error("closing body", err)
	}
}
