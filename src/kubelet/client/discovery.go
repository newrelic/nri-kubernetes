package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// discoverer implements Discoverer interface by using official Kubernetes' Go client
type discoverer struct {
	apiClient   client.Kubernetes
	logger      *logrus.Logger
	connChecker connectionChecker
	nodeName    string
}

const (
	healthzPath                = "/healthz"
	defaultInsecureKubeletPort = 10255
	defaultSecureKubeletPort   = 10250
)

// client type (if you need to add new values, do it at the end of the list)
const (
	httpBasic = iota
	httpInsecure
	httpSecure
)

// kubelet implements Client interface
type kubelet struct {
	httpClient *http.Client
	endpoint   url.URL
	config     rest.Config
	nodeIP     string
	nodeName   string
	httpType   int // httpBasic, httpInsecure, httpSecure
	logger     *logrus.Logger
}

type connectionParams struct {
	url      url.URL
	client   *http.Client
	httpType int // httpBasic, httpInsecure, httpSecure
}

type connectionChecker func(client *http.Client, URL url.URL, path, token string) error

func (c *kubelet) NodeIP() string {
	return c.nodeIP
}

// Do method calls discovered kubelet endpoint with specified method and path, i.e. "/stats/summary
func (c *kubelet) Do(method, path string) (*http.Response, error) {
	e := c.endpoint
	e.Path = filepath.Join(c.endpoint.Path, path)

	var r *http.Request
	var err error

	// TODO Create a new discoverer and client for cadvisor
	if path == metric.KubeletCAdvisorMetricsPath {
		if port := os.Getenv("CADVISOR_PORT"); port != "" {
			// We force to call the standalone cadvisor because k8s < 1.7.6 do not have /metrics/cadvisor kubelet endpoint.
			e.Scheme = "http"
			e.Host = fmt.Sprintf("%s:%s", c.nodeIP, port)
			e.Path = metric.StandaloneCAdvisorMetricsPath

			c.logger.Debugf("Using standalone cadvisor on port %s", port)
		}

		r, err = prometheus.NewRequest(method, e.String())
	} else {
		r, err = http.NewRequest(method, e.String(), nil)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating %s request to: %s. Got error: %s ", method, e.String(), err)
	}

	if c.endpoint.Scheme == "https" {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.BearerToken))
	}

	c.logger.Debugf("Calling Kubelet endpoint: %s", r.URL.String())

	return c.httpClient.Do(r)
}

func (sd *discoverer) Discover(timeout time.Duration) (client.HTTPClient, error) {
	node, err := sd.getNode(sd.nodeName)
	if err != nil {
		return nil, err
	}

	hostIP, err := getHostIP(node)
	if err != nil {
		return nil, err
	}

	port, err := getPort(node)
	if err != nil {
		return nil, err
	}

	hostURL := fmt.Sprintf("%s:%d", hostIP, port)

	connectionAPIHTTPS, secErr := sd.connectionAPIHTTPS(sd.nodeName, timeout)

	usedConnectionCases := make([]connectionParams, 0)
	switch port {
	case defaultInsecureKubeletPort:
		usedConnectionCases = append(usedConnectionCases, connectionHTTP(hostURL, timeout), connectionAPIHTTPS)
	case defaultSecureKubeletPort:
		usedConnectionCases = append(usedConnectionCases, connectionHTTPS(hostURL, timeout), connectionAPIHTTPS)
	default:
		usedConnectionCases = append(usedConnectionCases, connectionHTTP(hostURL, timeout), connectionHTTPS(hostURL, timeout), connectionAPIHTTPS)
	}

	config := sd.apiClient.Config()
	apiURL, err := apiURLFromConfig(config)
	if err != nil {
		return nil, err
	}

	for _, c := range usedConnectionCases {

		if secErr != nil && c.url.Host == apiURL.Host {
			return nil, secErr
		}

		err = sd.connChecker(c.client, c.url, healthzPath, config.BearerToken)
		if err != nil {
			sd.logger.Debug(err.Error())
			continue
		}

		return newKubelet(hostIP, sd.nodeName, c.url, config.BearerToken, c.client, c.httpType, sd.logger), nil
	}
	return nil, err
}

func apiURLFromConfig(config *rest.Config) (u *url.URL, err error) {
	u, err = url.Parse(config.Host)
	if err != nil {
		err = fmt.Errorf("error parsing kubernetes api url from in cluster config. %s", err)
	}

	return
}

func newKubelet(nodeIP string, nodeName string, endpoint url.URL, bearerToken string, client *http.Client, httpType int, logger *logrus.Logger) *kubelet {
	return &kubelet{
		nodeIP: nodeIP,
		endpoint: url.URL{
			Host:   endpoint.Host,
			Path:   endpoint.Path,
			Scheme: endpoint.Scheme,
		},
		httpClient: client,
		httpType:   httpType,
		config: rest.Config{
			BearerToken: bearerToken,
		},
		nodeName: nodeName,
		logger:   logger,
	}
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
		httpType: httpInsecure,
	}
}

func (sd *discoverer) connectionAPIHTTPS(nodeName string, timeout time.Duration) (connectionParams, error) {
	secureClient, err := sd.apiClient.SecureHTTPClient(timeout)
	if err != nil {
		return connectionParams{}, err
	}

	apiURL, err := apiURLFromConfig(sd.apiClient.Config())
	if err != nil {
		return connectionParams{}, err
	}

	return connectionParams{
		url: url.URL{
			Host:   apiURL.Host,
			Path:   fmt.Sprintf("/api/v1/nodes/%s/proxy/", nodeName),
			Scheme: apiURL.Scheme,
		},
		client:   secureClient,
		httpType: httpSecure,
	}, nil
}

func checkCall(client *http.Client, URL url.URL, path, token string) error {
	URL.Path = filepath.Join(URL.Path, path)

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

// NewDiscoverer instantiates a new Discoverer
func NewDiscoverer(nodeName string, logger *logrus.Logger) (client.Discoverer, error) {
	if nodeName == "" {
		return nil, errors.New("nodeName is empty")
	}

	c, err := client.NewKubernetes()
	if err != nil {
		return nil, err
	}

	return &discoverer{
		nodeName:    nodeName,
		logger:      logger,
		connChecker: checkCall,
		apiClient:   c,
	}, nil
}

func (sd *discoverer) getNode(nodeName string) (*v1.Node, error) {
	var node = new(v1.Node)
	var err error
	// Get the containing node and discover the InternalIP and Kubelet port
	node, err = sd.apiClient.FindNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("could not find node named %q. %s", nodeName, err)
	}

	return node, nil
}

func getPort(node *v1.Node) (int, error) {
	port := int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)
	if port == 0 {
		return 0, fmt.Errorf("could not get Kubelet port")
	}

	return port, nil
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
