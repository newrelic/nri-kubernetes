package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

const (
	healthzPath                = "/healthz"
	defaultInsecureKubeletPort = 10255
	defaultSecureKubeletPort   = 10250
	defaultTimeout             = time.Millisecond * 5000
)

// client type (if you need to add new values, do it at the end of the list)
const (
	httpBasic = iota
	httpsInsecure
	https
)

// TODO refactor this interface
// HTTPClient allows to connect to the discovered Kubernetes services
type HTTPClient interface {
	Get(path string) (*http.Response, error)
}

// httpDoer is a simple interface encapsulating objects capable of making requests.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	// http is an HttpDoer that the Kubelet client will use to make requests.
	http httpDoer
	// TODO: Use a non-sdk logger
	logger   log.Logger
	nodeName string
	nodeIP   string

	//  TODO review old data migrated
	endpoint    url.URL
	bearerToken string
	httpType    int // httpBasic, httpsInsecure, https
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
func New(kc kubernetes.Interface, nodeName string, opts ...OptionFunc) (*Client, error) {
	k := &Client{
		logger:   log.New(false, io.Discard),
		nodeName: nodeName,
	}

	for i, opt := range opts {
		if err := opt(k); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	node, err := kc.CoreV1().Nodes().Get(context.Background(), k.nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting info for node %q: %w", nodeName, err)
	}

	hostIP, err := getHostIP(node)
	if err != nil {
		return nil, err
	}

	k.nodeIP = hostIP

	port := int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)
	if port == 0 {
		return nil, fmt.Errorf("could not get Kubelet port")
	}

	hostURL := fmt.Sprintf("%s:%d", hostIP, port)

	usedConnectionCases := make([]connectionParams, 0)
	switch port {
	case defaultInsecureKubeletPort:
		usedConnectionCases = append(usedConnectionCases, connectionHTTP(hostURL, defaultTimeout))
	case defaultSecureKubeletPort:
		usedConnectionCases = append(usedConnectionCases, connectionHTTPS(hostURL, defaultTimeout))
	default:
		usedConnectionCases = append(usedConnectionCases, connectionHTTP(hostURL, defaultTimeout), connectionHTTPS(hostURL, defaultTimeout))
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("retrieving cluster config %w", err)
	}

	k.bearerToken = config.BearerToken

	for _, c := range usedConnectionCases {
		err = checkCall(c.client, c.url, healthzPath, config.BearerToken)
		if err != nil {
			k.logger.Debugf("trying connecting to kubelet: %s", err.Error())
			continue
		}

		k.endpoint = url.URL{
			Host:   c.url.Host,
			Path:   c.url.Path,
			Scheme: c.url.Scheme,
		}
		k.httpType = c.httpType
		k.http = c.client
	}

	connectionAPIHTTPS, err := connectionAPIHTTPS(kc, config.Host, nodeName)
	if err != nil {
		return nil, fmt.Errorf("trying connecting thorugh api server to kubelet: %w", err)
	}

	err = checkCall(connectionAPIHTTPS.client, connectionAPIHTTPS.url, healthzPath, config.BearerToken)
	if err != nil {
		return nil, fmt.Errorf("testing connection thorugh API: %w", err)
	}
	return k, nil
}

type connectionParams struct {
	url      url.URL
	client   httpDoer
	httpType int // httpBasic, httpsInsecure, https
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (c *Client) Get(urlPath string) (*http.Response, error) {
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	var r *http.Request
	var err error

	// TODO Create a new discoverer and client for cadvisor
	if urlPath == metric.KubeletCAdvisorMetricsPath {
		if port := os.Getenv("CADVISOR_PORT"); port != "" {
			// We force to call the standalone cadvisor because k8s < 1.7.6 do not have /metrics/cadvisor kubelet endpoint.
			e.Scheme = "http"
			e.Host = fmt.Sprintf("%s:%s", c.nodeIP, port)
			e.Path = metric.StandaloneCAdvisorMetricsPath

			c.logger.Debugf("Using standalone cadvisor on port %s", port)
		}

		r, err = prometheus.NewRequest(e.String())
	} else {
		r, err = http.NewRequest(http.MethodGet, e.String(), nil)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %s ", e.String(), err)
	}

	if c.endpoint.Scheme == "https" {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	}

	c.logger.Debugf("Calling Kubelet endpoint: %s", r.URL.String())

	return c.http.Do(r)
}

func connectionHTTP(host string, timeout time.Duration) connectionParams {
	return connectionParams{
		url: url.URL{
			Host:   host,
			Scheme: "http",
		},
		client:   client.BasicHTTPClient(timeout),
		httpType: httpBasic,
	}
}

func connectionHTTPS(host string, timeout time.Duration) connectionParams {
	return connectionParams{
		url: url.URL{
			Host:   host,
			Scheme: "https",
		},
		client:   client.InsecureHTTPClient(timeout),
		httpType: httpsInsecure,
	}
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

func connectionAPIHTTPS(kc kubernetes.Interface, hostname string, nodeName string) (connectionParams, error) {
	client, err := GetClientFromRestInterface(kc)
	if err != nil {
		err = fmt.Errorf("error getting client from rest client interface: %w", err)
	}

	apiURL, err := url.Parse(hostname)
	if err != nil {
		err = fmt.Errorf("error parsing kubernetes api url from in cluster config: %w", err)
	}

	return connectionParams{
		url: url.URL{
			Host:   apiURL.Host,
			Path:   fmt.Sprintf("/api/v1/nodes/%s/proxy/", nodeName),
			Scheme: apiURL.Scheme,
		},
		client:   client,
		httpType: https,
	}, nil
}

func checkCall(client httpDoer, URL url.URL, urlPath, token string) error {
	URL.Path = path.Join(URL.Path, urlPath)

	r, err := http.NewRequest(http.MethodGet, URL.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request to: %s. Got error: %s ", URL.String(), err)
	}
	if URL.Scheme == "https" {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	resp, err := client.Do(r)
	if err != nil {
		return fmt.Errorf("error trying to connect to: %s. Got error: %s ", URL.String(), err)
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("error calling endpoint %s. Got status code: %d", URL.String(), resp.StatusCode)
}

func getHostIP(node *v1.Node) (string, error) {
	var ip string

	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			ip = address.Address
			break
		}
	}

	if ip == "" {
		return "", fmt.Errorf("could not get Kubelet host IP")
	}

	return ip, nil
}
