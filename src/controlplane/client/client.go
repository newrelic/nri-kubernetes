package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// Client implements a client for ControlPlane component.
type Client struct {
	// TODO: Use a non-sdk logger
	logger   log.Logger
	doer     client.HTTPDoer
	endpoint url.URL
}

type OptionFunc func(c *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(c *Client) error {
		if logger == nil {
			return fmt.Errorf("logger canont be nil")
		}

		c.logger = logger

		return nil
	}
}

// New builds a Client using the given options.
func New(connector Connector, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger: log.New(false, io.Discard),
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	if connector == nil {
		return nil, fmt.Errorf("connector should not be nil")
	}

	conn, err := connector.Connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to component using the connector: %w", err)
	}

	c.doer = conn.client
	c.endpoint = conn.url

	return c, nil
}

func (c *Client) Get(urlPath string) (*http.Response, error) {
	req, err := prometheus.NewRequest(c.endpoint.String())
	if err != nil {
		return nil, fmt.Errorf("creating request to: %q. Got error: %v ", c.endpoint.String(), err)
	}

	c.logger.Debugf("http request created with url: %q", req.URL.String())

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting response from: %q. Got error: %w ", req.URL.String(), err)
	}

	return resp, nil
}
