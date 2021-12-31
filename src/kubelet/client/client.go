package client

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/newrelic/nri-kubernetes/v2/src/common"
)

const (
	healthzPath      = "/healthz"
	podsPath         = "/pods"
	cadvisorPath     = "/metrics/cadvisor"
	statsSummaryPath = "/stats/summary"
)

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	client common.HTTPClient
	url    url.URL
}

func NewClient(url url.URL, doer common.HTTPDoer) *Client {
	return &Client{
		url:    url,
		client: common.NewHTTP(doer),
	}
}

// Probe attempts to connect to the healthz endpoint of the kubelet and returns any error that occurs.
func (c *Client) Probe() error {
	rc, err := c.client.Get(strings.Trim(c.url.String(), "/") + healthzPath)
	if err != nil {
		return err
	}

	defer rc.Body.Close()

	return nil
}

// Pods return the contents of the /pods endpoint.
func (c *Client) Pods() (*http.Response, error) {
	return c.client.Get(strings.Trim(c.url.String(), "/") + podsPath)
}

// MetricsCadvisor return the contents of the /metrics/cadvisor endpoint.
func (c *Client) MetricsCadvisor() (*http.Response, error) {
	return c.client.Get(strings.Trim(c.url.String(), "/") + cadvisorPath)
}

// StatsSummary return the contents of the /stats/summary endpoint.
func (c *Client) StatsSummary() (*http.Response, error) {
	return c.client.Get(strings.Trim(c.url.String(), "/") + statsSummaryPath)
}
