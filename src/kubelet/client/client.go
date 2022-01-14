package client

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v2/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

const (
	healthzPath             = "/healthz"
	defaultHTTPKubeletPort  = 10255
	defaultHTTPSKubeletPort = 10250
	defaultTimeout          = time.Millisecond * 5000
)

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	logger   *log.Logger
	doer     client.HTTPDoer
	endpoint url.URL
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.logger = logger
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

	var doer client.HTTPDoer
	if client, ok := conn.client.(*http.Client); ok {
		httpPester := pester.NewExtendedClient(client)
		httpPester.Backoff = pester.LinearBackoff
		//Note that httpPester.Timeout overwrites the timeout inside the client
		httpPester.Timeout = defaultTimeout
		httpPester.MaxRetries = 3
		httpPester.LogHook = func(e pester.ErrEntry) {
			c.logger.Debugf("getting data from kubelet: %v", e)
		}
		doer = httpPester
	} else {
		c.logger.Debugf("running kubelet client without pester")
		doer = conn.client
	}

	c.doer = doer
	c.endpoint = conn.url

	return c, nil
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (c *Client) Get(urlPath string) (*http.Response, error) {
	// Notice that this is the client to interact with kubelet. In case of CAdvisor the MetricFamiliesGetFunc is used
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := http.NewRequest(http.MethodGet, e.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %s ", e.String(), err)
	}

	c.logger.Debugf("Calling Kubelet endpoint: %s", r.URL.String())

	return c.doer.Do(r)
}

// MetricFamiliesGetFunc returns a function that obtains metric families from a list of prometheus queries.
func (c *Client) MetricFamiliesGetFunc(url string) prometheus.FetchAndFilterMetricsFamilies {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		e := c.endpoint
		e.Path = path.Join(c.endpoint.Path, url)

		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, e.String(), queries, c.logger)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families %q: %w", e.String(), err)
		}

		return mFamily, nil
	}
}
