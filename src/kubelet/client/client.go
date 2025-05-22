package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const (
	healthzPath             = "/healthz"
	defaultHTTPKubeletPort  = 10255
	defaultHTTPSKubeletPort = 10250
)

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	logger   *log.Logger
	doer     client.HTTPDoer
	endpoint url.URL
	retries  int
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.logger = logger
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
func New(connector Connector, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger: logutil.Discard,
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
		return nil, fmt.Errorf("connecting to kubelet using the connector: %w", err)
	}

	if client, ok := conn.client.(*http.Client); ok {
		httpPester := pester.NewExtendedClient(client)
		httpPester.Backoff = pester.LinearBackoff
		httpPester.MaxRetries = c.retries
		httpPester.LogHook = func(e pester.ErrEntry) {
			c.logger.Debugf("getting data from kubelet: %v", e)
		}
		c.doer = httpPester
	} else {
		c.logger.Debugf("running kubelet client without pester")
		c.doer = conn.client
	}

	c.endpoint = conn.url

	return c, nil
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (client *Client) Get(urlPath string) (*http.Response, error) {
	// Notice that this is the client to interact with kubelet. In case of CAdvisor the MetricFamiliesGetFunc is used
	e := client.endpoint
	e.Path = path.Join(client.endpoint.Path, urlPath)

	result, err := client.GetURI(e)

	if err != nil {
		return nil, fmt.Errorf("error getting path %s: %w ", urlPath, err)
	}

	return result, nil
}

func (client *Client) GetURI(uri url.URL) (*http.Response, error) {
	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %w ", uri.String(), err)
	}

	client.logger.Debugf("Calling Kubelet endpoint: %s", r.URL.String())

	result, err := client.doer.Do(r)
	if err != nil {
		return nil, fmt.Errorf("error getting url %s: %w ", uri.String(), err)
	}
	return result, nil
}

// MetricFamiliesGetFunc returns a function that obtains metric families from a list of prometheus queries.
func (client *Client) MetricFamiliesGetFunc(url string) prometheus.FetchAndFilterMetricsFamilies {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		e := client.endpoint
		e.Path = path.Join(client.endpoint.Path, url)

		mFamily, err := prometheus.GetFilteredMetricFamilies(client.doer, e.String(), queries, client.logger)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families %q: %w", e.String(), err)
		}

		return mFamily, nil
	}
}
