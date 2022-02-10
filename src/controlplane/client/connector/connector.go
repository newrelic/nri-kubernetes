package connector

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/authenticator"
)

const (
	defaultMetricsPath = "/metrics"
)

// Connector provides an interface to retrieve connParams to connect to a Control Plane instance.
type Connector interface {
	// Connect probes the connector endpoints and returns connParams to scrape a valid endpoint.
	Connect() (*ConnParams, error)
}

// ConnParams contains the authenticated parameters to scrape an endpoint.
type ConnParams struct {
	URL    url.URL
	Client client.HTTPDoer
}

type Config struct {
	Authenticator authenticator.Authenticator
	Endpoints     []config.Endpoint
	Timeout       time.Duration
}

type OptionFunc func(dc *DefaultConnector) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(dc *DefaultConnector) error {
		dc.logger = logger
		return nil
	}
}

// DefaultConnector implements Connector interface for the Control Plane components.
type DefaultConnector struct {
	Config
	logger *log.Logger
}

// New returns a DefaultConnector.
func New(config Config, opts ...OptionFunc) (*DefaultConnector, error) {
	dc := &DefaultConnector{
		Config: config,
		logger: logutil.Discard,
	}

	for i, opt := range opts {
		if err := opt(dc); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return dc, nil
}

// Connect iterates over the endpoints list probing each endpoint with a HEAD request
// and returns the connection parameters of the first endpoint that respond Status OK.
func (dp *DefaultConnector) Connect() (*ConnParams, error) {
	for _, e := range dp.Endpoints {
		dp.logger.Debugf("Configuring endpoint %q for probing", e.URL)

		u, err := url.Parse(e.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing endpoint url %q: %w", e.URL, err)
		}

		if strings.TrimSuffix(u.Path, "/") == "" {
			dp.logger.Debugf("Autodiscover endpoint %q does not contain path, adding default %q", e.URL, defaultMetricsPath)
			u.Path = defaultMetricsPath
		}

		rt, err := dp.Authenticator.AuthenticatedTransport(e)
		if err != nil {
			return nil, fmt.Errorf("creating HTTP client for endpoint %q: %w", e.URL, err)
		}

		httpClient := &http.Client{Timeout: dp.Timeout, Transport: rt}

		if err := dp.probe(u.String(), httpClient); err != nil {
			dp.logger.Debugf("Endpoint %q probe failed, skipping: %v", e.URL, err)
			continue
		}

		dp.logger.Debugf("Endpoint %q probed successfully", e.URL)

		return &ConnParams{URL: *u, Client: httpClient}, nil
	}

	return nil, fmt.Errorf("all endpoints in the list failed to respond")
}

// probe executes a HEAD request to the endpoint and fails if the response code
// is not StatusOK.
func (dp *DefaultConnector) probe(endpoint string, client *http.Client) error {
	resp, err := client.Head(endpoint)
	if err != nil {
		return fmt.Errorf("http HEAD request failed: %w", err)
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status: %v", resp.Status)
	}

	return nil
}
