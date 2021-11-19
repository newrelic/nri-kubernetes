package client

import (
	"fmt"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"io"
	"k8s.io/client-go/kubernetes"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"k8s.io/client-go/rest"
)

const (
	healthzPath             = "/healthz"
	defaultHTTPKubeletPort  = 10255
	defaultHTTPSKubeletPort = 10250
	defaultTimeout          = time.Millisecond * 5000
)

// TODO refactor this interface
// HTTPGetter allows to connect to the discovered Kubernetes services
type HTTPGetter interface {
	Get(path string) (*http.Response, error)
}

// httpDoer is a simple interface encapsulating objects capable of making requests.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	// TODO: Use a non-sdk logger
	logger      log.Logger
	doer        httpDoer
	endpoint    url.URL
	bearerToken string
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
func New(kc kubernetes.Interface, nodeName string, nodeIP string, inClusterConfig *rest.Config, opts ...OptionFunc) (*Client, error) {
	c := &Client{
		logger: log.New(false, io.Discard),
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	err, kubeletPort, client, err2 := getKubeletPort(kc, nodeName)
	if err2 != nil {
		return client, err2
	}

	c.bearerToken = inClusterConfig.BearerToken

	err = c.setupLocalConnection(nodeIP, kubeletPort)
	if err == nil {
		c.logger.Debugf("connected to Kubelet directly with nodeIP")
		return c, nil
	}
	c.logger.Debugf("Kubelet connection with nodeIP failed: %v", err)

	err = c.setupConnectionAPI(kc, inClusterConfig.Host, nodeName)
	if err != nil {
		return nil, fmt.Errorf("no connection method was succeeded: %w ", err)
	}
	c.logger.Debugf("connected to Kubelet with API proxy")

	return c, nil
}

func (c *Client) setupLocalConnection(nodeIP string, portInt int32) error {
	c.logger.Debugf("trying connecting to kubelet directly with nodeIP")

	port := fmt.Sprintf("%d", portInt)
	hostURL := net.JoinHostPort(nodeIP, port)

	var connToTest []connParams
	switch portInt {
	case defaultHTTPKubeletPort:
		connToTest = []connParams{connectionHTTP(hostURL, defaultTimeout)}
	case defaultHTTPSKubeletPort:
		connToTest = []connParams{connectionHTTPS(hostURL, defaultTimeout)}
	default:
		// In case the kubelet port is not the standard one, we do not know if the schema is HTTP or HTTPS and
		// we need to test both
		connToTest = []connParams{connectionHTTP(hostURL, defaultTimeout), connectionHTTPS(hostURL, defaultTimeout)}
	}

	for _, conn := range connToTest {
		err := checkCall(conn, c.bearerToken)
		if err != nil {
			c.logger.Debugf("trying connecting to kubelet: %s", err.Error())
			continue
		}

		c.doer = conn.client
		c.endpoint = conn.url

		return nil
	}

	return fmt.Errorf("no connection succeded through localhost")
}

func (c *Client) setupConnectionAPI(kc kubernetes.Interface, apiServer string, nodeName string) error {
	c.logger.Debugf("trying connecting to kubelet directly with API proxy")

	err, conn := connectionAPIProxy(kc, apiServer, nodeName)
	if err != nil {
		err = fmt.Errorf("creating connection parameters for API proxy: %w", err)
	}

	err = checkCall(conn, c.bearerToken)
	if err != nil {
		return fmt.Errorf("testing connection thorugh API: %w", err)
	}

	c.endpoint = conn.url
	c.doer = conn.client

	return nil
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (c *Client) Get(urlPath string) (*http.Response, error) {
	// Notice that this is the client to interact with kubelet. In case of CAdvisor the prometheus.Do is used

	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := http.NewRequest(http.MethodGet, e.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %s ", e.String(), err)
	}

	if c.endpoint.Scheme == "https" {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	}

	c.logger.Debugf("Calling Kubelet endpoint: %s", r.URL.String())

	return c.doer.Do(r)
}

// MetricFamiliesGetter is the interface satisfied by Client.
// TODO: This whole flow is too convoluted, we should refactor and rename this.
type MetricFamiliesGetter interface {
	// MetricFamiliesGetter returns a prometheus.FilteredFetcher configured to get KSM metrics from and endpoint.
	// prometheus.FilteredFetcher will be used by the prometheus client to scrape and filter metrics.
	MetricFamiliesGetter(url string) prometheus.MetricsFamiliesGetter
}

// MetricFamiliesGetter returns a function that obtains metric families from a list of prometheus queries.
func (c *Client) MetricFamiliesGetter(url string) prometheus.MetricsFamiliesGetter {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		e := c.endpoint
		e.Path = path.Join(c.endpoint.Path, url)

		headers := map[string]string{}
		if c.endpoint.Scheme == "https" {
			headers["Authorization"] = fmt.Sprintf("Bearer %s", c.bearerToken)
		}

		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, headers, e.String(), queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families %q: %w", e.String(), err)
		}

		return mFamily, nil
	}
}
