package connector

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client/authenticator"
)

const (
	DefaultTimout      = 5000 * time.Millisecond
	defaultMetricsPath = "/metrics"
)

// Connector provides an interface to retrieve []connParams to connect to a Control Plane instance.
type Connector interface {
	Connect() (*ConnParams, error)
}

type ConnParams struct {
	URL    url.URL
	Client client.HTTPDoer
}

type defaultConnector struct {
	// TODO: Use a non-sdk logger
	logger        log.Logger
	authenticator authenticator.Authenticator
	endpoints     []config.Endpoint
}

// DefaultConnector returns a defaultConnector that probes all endpoints in the list and return the first responding status OK.
func DefaultConnector(endpoints []config.Endpoint, authenticator authenticator.Authenticator, logger log.Logger) Connector {
	return &defaultConnector{
		logger:        logger,
		authenticator: authenticator,
		endpoints:     endpoints,
	}
}

// Connect iterates over the endpoints list probing each endpoint with a HEAD request
// and returns the connection parameters of the first endpoint that respond Status OK.
func (dp *defaultConnector) Connect() (*ConnParams, error) {
	for _, e := range dp.endpoints {
		dp.logger.Debugf("Configuring endpoint %q for probing", e.URL)

		u, err := url.Parse(e.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing endpoint url %q: %w", e.URL, err)
		}

		if strings.TrimSuffix(u.Path, "/") == "" {
			dp.logger.Debugf("Autodiscover endpoint %q does not contain path, adding default %q", e.URL, defaultMetricsPath)
			u.Path = defaultMetricsPath
		}

		rt, err := dp.authenticator.AuthenticatedTransport(e)
		if err != nil {
			return nil, fmt.Errorf("creating HTTP client for endpoint %q: %w", e.URL, err)
		}

		httpClient := &http.Client{Timeout: DefaultTimout, Transport: rt}

		if err := dp.probeEndpoint(u.String(), httpClient); err != nil {
			dp.logger.Debugf("Endpoint %q probe failed, skipping: %v", e.URL, err)
			continue
		}

		dp.logger.Debugf("Endpoint %q probed successfully", e.URL)

		return &ConnParams{URL: *u, Client: httpClient}, nil
	}

	return nil, fmt.Errorf("all endpoints in the list failed to response")
}

// probeEndpoint executes a HEAD request to the url and fails if the response code
// is not StatusOK.
func (dp *defaultConnector) probeEndpoint(url string, client *http.Client) error {
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("http HEAD request failed: %w", err)
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status: %v", resp.Status)
	}

	return nil
}
