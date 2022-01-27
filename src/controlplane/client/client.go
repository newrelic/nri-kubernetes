package client

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/connector"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

// Client implements a client for ControlPlane component.
type Client struct {
	logger   *log.Logger
	doer     client.HTTPDoer
	endpoint url.URL
	retries  int
}

type OptionFunc func(c *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(c *Client) error {
		if logger == nil {
			return fmt.Errorf("logger canont be nil")
		}

		c.logger = logger

		return nil
	}
}

// WithMaxRetries returns an OptionFunc to change the number of retries used int Pester Client.
func WithMaxRetries(retries int) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.retries = retries
		return nil
	}
}

// New builds a Client using the given options.
func New(connector connector.Connector, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger: logutil.Discard,
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	conn, err := connector.Connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to component using the connector: %w", err)
	}

	if client, ok := conn.Client.(*http.Client); ok {
		httpPester := pester.NewExtendedClient(client)
		httpPester.Backoff = pester.LinearBackoff
		httpPester.MaxRetries = c.retries
		httpPester.LogHook = func(e pester.ErrEntry) {
			c.logger.Debugf("getting data from control plane: %v", e)
		}
		c.doer = httpPester
	} else {
		c.logger.Debugf("running control plane client without pester")
		c.doer = conn.Client
	}

	c.endpoint = conn.URL

	return c, nil
}

// MetricFamiliesGetFunc returns a function that obtains metric families from a list of prometheus queries.
// Notice that it does not satisfy prometheus.MetricFamiliesGetFunc, since the url path is injected by the connector
func (c *Client) MetricFamiliesGetFunc() prometheus.FetchAndFilterMetricsFamilies {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, c.endpoint.String(), queries, c.logger)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families %q: %w", c.endpoint.String(), err)
		}

		return mFamily, nil
	}
}
