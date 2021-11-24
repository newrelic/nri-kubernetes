package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
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
	// TODO: Use a non-sdk logger
	logger    log.Logger
	doer      client.HTTPDoer
	endpoint  url.URL
	connector Connector
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.logger = logger
		return nil
	}
}

// WithCustomConnector returns an OptionFunc to change the default connector.
func WithCustomConnector(connector Connector) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.connector = connector
		return nil
	}
}

// New builds a Client using the given options.
func New(kc kubernetes.Interface, config config.Mock, inClusterConfig *rest.Config, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger: log.New(false, io.Discard),
		connector: defaultConnector{
			logger:             log.NewStdErr(config.Verbose),
			apiServerHost:      inClusterConfig.Host,
			kc:                 kc,
			tripperBearerToken: tripperWithBearerToken(inClusterConfig.BearerToken),
			config:             config,
		},
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	conn, err := c.connector.connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to kubelet using the connector: %w", err)
	}

	c.doer = conn.client
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
