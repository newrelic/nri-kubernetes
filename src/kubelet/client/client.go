package client

import (
	"fmt"
	"io"
	"net"
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
	logger        log.Logger
	doer          client.HTTPDoer
	endpoint      url.URL
	apiServerHost string
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(kc *Client) error {
		kc.logger = logger
		return nil
	}
}

// New builds a Client using the given options.
func New(kc kubernetes.Interface, config config.Mock, inClusterConfig *rest.Config, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger:        log.New(false, io.Discard),
		apiServerHost: inClusterConfig.Host,
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	conn, err := c.setupConnection(kc, tripperWithBearerToken(inClusterConfig.BearerToken), config)

	if err != nil {
		return nil, fmt.Errorf("connecting to kubelet: %w", err)
	}

	c.doer = conn.client
	c.endpoint = conn.url

	return c, nil
}

func (c *Client) setupConnection(kc kubernetes.Interface, tripperBearerToken http.RoundTripper, config config.Mock) (*connParams, error) {
	kubeletPort, err := getKubeletPort(kc, config.NodeName)
	if err != nil {
		return nil, fmt.Errorf("getting kubelet port: %w", err)
	}

	conn, err := c.setupLocalConnection(tripperBearerToken, config.NodeIP, kubeletPort)
	if err == nil {
		c.logger.Debugf("connected to Kubelet directly with nodeIP")
		return conn, nil
	}

	c.logger.Debugf("Kubelet connection with nodeIP failed: %v", err)
	c.logger.Debugf("Connecting to kubelet directly with API proxy")

	conn, err = checkConnectionAPIProxy(c.apiServerHost, config.NodeName, tripperBearerToken)
	if err != nil {
		return nil, fmt.Errorf("creating connection parameters for API proxy: %w", err)
	}

	return conn, nil
}

func (c *Client) setupLocalConnection(tripperWithBearerToken http.RoundTripper, nodeIP string, portInt int32) (*connParams, error) {
	c.logger.Debugf("connecting to kubelet directly with nodeIP")
	var err error

	port := fmt.Sprintf("%d", portInt)
	hostURL := net.JoinHostPort(nodeIP, port)

	var conn *connParams

	switch portInt {
	case defaultHTTPKubeletPort:
		if conn, err = checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}

	case defaultHTTPSKubeletPort:
		if conn, err = checkConnectionHTTPS(hostURL, tripperWithBearerToken); err == nil {
			return conn, nil
		}

	default:
		// The port is not a standard one and we need to check both schemas.
		if conn, err = checkConnectionHTTPS(hostURL, tripperWithBearerToken); err == nil {
			return conn, nil
		}

		if conn, err = checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("no connection succeeded through localhost: %w", err)
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
