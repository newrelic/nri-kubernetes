package client

import (
	"fmt"
	"net/http"

	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type Client struct {
	logger           log.Logger
	connectionParams *connParams
}

type Config struct {
	Logger    log.Logger
	Connector Connector
}

func New(cfg Config) (client.HTTPClient, error) {
	if cfg.Connector == nil {
		return nil, fmt.Errorf("connector must not be nil")
	}

	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger must not be nil")
	}

	cp, err := cfg.Connector.Connect()
	if err != nil {
		return nil, err
	}

	c := &Client{
		logger:           cfg.Logger,
		connectionParams: cp,
	}

	return c, nil
}

func (c *Client) Get(urlPath string) (*http.Response, error) {
	req, err := prometheus.NewRequest(c.connectionParams.url.String())
	if err != nil {
		return nil, fmt.Errorf("creating request to: %q. Got error: %v ", c.connectionParams.url.String(), err)
	}

	c.logger.Debugf("http request created with url: %q", req.URL.String())

	resp, err := c.connectionParams.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting response from: %q. Got error: %w ", req.URL.String(), err)
	}

	return resp, nil
}
