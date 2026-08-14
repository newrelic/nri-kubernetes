package cloud

import (
	"fmt"
	"io"
	"net/http"

	"github.com/sethgrid/pester"
)

// Metadata endpoints. Declared as vars so tests can point them at httptest servers.
var (
	gkeMetadataBaseURL = "http://metadata.google.internal/computeMetadata/v1"
)

const maxMetadataBodyBytes = 1 * 1024 * 1024

// newMetadataClient returns a pester client with a short timeout suitable for
// link-local metadata endpoints.
func newMetadataClient() *pester.Client {
	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = 2
	c.Timeout = defaultHTTPTimeout
	return c
}

// doMetadataGet executes req and returns the trimmed body for a 200 response.
func doMetadataGet(req *http.Request) (string, error) {
	resp, err := newMetadataClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBodyBytes))
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}
