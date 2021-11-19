package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// DataClient allows to connect to the discovered Kubernetes services
type DataClient interface {
	Get(path string) (*http.Response, error)
	// MetricFamiliesGetter returns a prometheus.FilteredFetcher configured to get KSM metrics from and endpoint.
	// prometheus.FilteredFetcher will be used by the prometheus client to scrape and filter metrics.
	MetricFamiliesGetter(url string) prometheus.MetricsFamiliesGetter
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

func getKubeletPort(kc kubernetes.Interface, nodeName string) (error, int32, *Client, error) {
	//We pay the price of a single call getting a node to avoid asking the user the Kubelet port if different from the standard one

	node, err := kc.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, 0, nil, fmt.Errorf("getting info for node %q: %w", nodeName, err)
	}
	kubeletPort := node.Status.DaemonEndpoints.KubeletEndpoint.Port
	return err, kubeletPort, nil, nil
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

type connParams struct {
	url    url.URL
	client httpDoer
}

func connectionHTTP(host string, timeout time.Duration) connParams {
	return connParams{
		url: url.URL{
			Host:   host,
			Scheme: "http",
		},
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func connectionHTTPS(host string, timeout time.Duration) connParams {
	client := &http.Client{
		Timeout: timeout,
	}
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return connParams{
		url: url.URL{
			Host:   host,
			Scheme: "https",
		},
		client: client,
	}
}

func connectionAPIProxy(kc kubernetes.Interface, apiServer string, nodeName string) (error, connParams) {
	client, err := GetClientFromRestInterface(kc)
	if err != nil {
		err = fmt.Errorf("getting client from rest client interface: %w", err)
	}

	apiURL, err := url.Parse(apiServer)
	if err != nil {
		err = fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	conn := connParams{
		url: url.URL{
			Host:   apiURL.Host,
			Path:   fmt.Sprintf("/api/v1/nodes/%s/proxy/", nodeName),
			Scheme: apiURL.Scheme,
		},
		client: client,
	}
	return err, conn
}

// GetClientFromInterface it merely an helper to allow using the fake client
var GetClientFromRestInterface = getClientFromRestInterface

func getClientFromRestInterface(kc kubernetes.Interface) (httpDoer, error) {
	secureClient, ok := kc.Discovery().RESTClient().(*rest.RESTClient)
	if !ok {
		return nil, errors.New("failed to set up a client for connecting to Kubelet through API proxy")
	}
	return secureClient.Client, nil
}

func checkCall(conn connParams, token string) error {
	conn.url.Path = path.Join(conn.url.Path, healthzPath)

	r, err := http.NewRequest(http.MethodGet, conn.url.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request to: %s. Got error: %s ", conn.url.String(), err)
	}

	if conn.url.Scheme == "https" {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := conn.client.Do(r)
	if err != nil {
		return fmt.Errorf("error trying to connect to: %s. Got error: %s ", conn.url.String(), err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return fmt.Errorf("error calling endpoint %s. Got status code: %d", conn.url.String(), resp.StatusCode)
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

// MetricFamiliesGetter returns a function that obtains metric families from a list of prometheus queries.
func (c *Client) MetricFamiliesGetter(url string) prometheus.MetricsFamiliesGetter {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, url, queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}
