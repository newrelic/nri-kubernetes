package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
	httpSchema   = "http"
	httpsSchema  = "https"
)

// Connector provides an interface to retrieve connParams to connect to a Kubelet instance.
type Connector interface {
	Connect() (*connParams, error)
}

type defaultConnector struct {
	// TODO: Use a non-sdk logger
	logger          log.Logger
	kc              kubernetes.Interface
	inClusterConfig *rest.Config
	config          *config.Mock
}

// DefaultConnector returns a defaultConnector that checks connection against local kubelet and api proxy.
func DefaultConnector(kc kubernetes.Interface, config *config.Mock, inClusterConfig *rest.Config, logger log.Logger) Connector {
	return &defaultConnector{
		logger:          logger,
		inClusterConfig: inClusterConfig,
		kc:              kc,
		config:          config,
	}
}

func (dp *defaultConnector) Connect() (*connParams, error) {

	kubeletPort, err := dp.getKubeletPort()
	if err != nil {
		return nil, fmt.Errorf("getting kubelet port: %w", err)
	}

	kubeletSchema := dp.getKubeletSchema(kubeletPort)
	hostURL := net.JoinHostPort(dp.config.NodeIP, fmt.Sprint(kubeletPort))

	dp.logger.Infof("Trying to connect to kubelet locally with schema=%q hostURL=%q", kubeletSchema, hostURL)

	conn, err := dp.checkLocalConnection(tripperWithBearerToken(dp.inClusterConfig.BearerToken), kubeletSchema, hostURL)
	if err == nil {
		dp.logger.Infof("Connected to Kubelet through nodeIP with schema=%q hostURL=%q", kubeletSchema, hostURL)
		return conn, nil
	}
	dp.logger.Infof("Kubelet not reachable locally with schema=%q hostURL=%q: %v", kubeletSchema, hostURL, err)
	dp.logger.Infof("Trying to connect to kubelet through API proxy %q to node %q", dp.inClusterConfig.Host, dp.config.NodeName)

	tripperAPI, err := rest.TransportFor(dp.inClusterConfig)
	if err != nil {
		return nil, fmt.Errorf("creating tripper connecting to kubelet through API server proxy: %w", err)
	}
	conn, err = dp.checkConnectionAPIProxy(dp.inClusterConfig.Host, dp.config.NodeName, tripperAPI)
	if err != nil {
		return nil, fmt.Errorf("creating connection parameters for API proxy: %w", err)
	}

	return conn, nil
}

func (dp *defaultConnector) setupLocalConnection(tripperWithBearerToken http.RoundTripper, schema string, hostURL string) (*connParams, error) {
	dp.logger.Debugf("connecting to kubelet directly with nodeIP")
	var err error
	var conn *connParams

	switch schema {
	case httpSchema:
		if conn, err = dp.checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}
	case httpsSchema:
		if conn, err = dp.checkConnectionHTTPS(hostURL, tripperWithBearerToken); err == nil {
			return conn, nil
		}
	default:
		dp.logger.Errorf("Checking both HTTP and HTTPS since the schema was not detected automatically, " +
			"you can set set kubelet.schema to avoid this behaviour")

		if conn, err = dp.checkConnectionHTTPS(hostURL, tripperWithBearerToken); err == nil {
			return conn, nil
		}

		if conn, err = dp.checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("no connection succeeded through localhost: %w", err)
}

func (dp *defaultConnector) getKubeletPort() (int32, error) {
	if dp.config.Kubelet.Port != 0 {
		dp.logger.Debugf("Setting Port %d as specified by user config", dp.config.Kubelet.Port)
		return dp.config.Kubelet.Port, nil
	}

	//We pay the price of a single call getting a node to avoid asking the user the Kubelet port if different from the standard one
	node, err := dp.kc.CoreV1().Nodes().Get(context.Background(), dp.config.NodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting node %q: %w", dp.config.NodeName, err)
	}

	port := node.Status.DaemonEndpoints.KubeletEndpoint.Port
	dp.logger.Debugf("Setting Port %d as found in status condition", port)

	return port, nil
}

func (dp *defaultConnector) getKubeletSchema(kubeletPort int32) string {
	if dp.config.Kubelet.Schema != "" {
		dp.logger.Debugf("Setting Kubelet Endpoint Schema %s as specified by user config", dp.config.Kubelet.Schema)
		return dp.config.Kubelet.Schema
	}

	switch kubeletPort {
	case defaultHTTPKubeletPort:
		dp.logger.Debugf("Setting Kubelet Endpoint Schema http since kubeletPort is %d", kubeletPort)
		return httpSchema
	case defaultHTTPSKubeletPort:
		dp.logger.Debugf("Setting Kubelet Endpoint Schema https since kubeletPort is %d", kubeletPort)
		return httpsSchema
	default:
		dp.logger.Errorf("Cannot automatically figure out schema from non-standard port %d, please set kubelet.schema in the config file.", kubeletPort)
		return ""
	}
}

type connParams struct {
	url    url.URL
	client client.HTTPDoer
}

func (dp *defaultConnector) checkConnectionAPIProxy(apiServer string, nodeName string, tripperAPIproxy http.RoundTripper) (*connParams, error) {
	apiURL, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	conn := connParams{
		client: &http.Client{
			Timeout:   defaultTimeout,
			Transport: tripperAPIproxy,
		},
		url: url.URL{
			Host:   apiURL.Host,
			Scheme: apiURL.Scheme,
			Path:   path.Join(fmt.Sprintf(apiProxyPath, nodeName)),
		},
	}

	dp.logger.Debugf("Testing kubelet connection through API proxy: %s%s", apiURL.Host, conn.url.Path)

	if err = checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTP(hostURL string) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection over plain http to %s", hostURL)

	conn := defaultConnParamsHTTP(hostURL)
	if err := checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTPS(hostURL string, tripperBearer http.RoundTripper) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection over https to %s", hostURL)

	conn := defaultConnParamsHTTPS(hostURL, tripperBearer)
	if err := checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func checkConnection(conn connParams) error {
	conn.url.Path = path.Join(conn.url.Path, healthzPath)

	r, err := http.NewRequest(http.MethodGet, conn.url.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request to %q : %s ", conn.url.String(), err)
	}

	resp, err := conn.client.Do(r)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w ", conn.url.String(), err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("calling %s got non-200 status code: %d", conn.url.String(), resp.StatusCode)
	}

	return nil
}

func tripperWithBearerToken(token string) http.RoundTripper {
	// Here we're using the default http.Transport configuration, but with a modified TLS config.
	// The DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig.InsecureSkipVerify = true
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// Use the default kubernetes Bearer token authentication RoundTripper
	tripperWithBearer := transport.NewBearerAuthRoundTripper(token, t)
	return tripperWithBearer
}

func defaultConnParamsHTTP(hostURL string) connParams {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	u := url.URL{
		Host:   hostURL,
		Scheme: httpSchema,
	}
	return connParams{u, httpClient}
}

func defaultConnParamsHTTPS(hostURL string, tripper http.RoundTripper) connParams {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	httpClient.Transport = tripper

	u := url.URL{
		Host:   hostURL,
		Scheme: httpsSchema,
	}
	return connParams{u, httpClient}
}

type fixedConnector struct {
	URL    url.URL
	Client client.HTTPDoer
}

func (mc *fixedConnector) Connect() (*connParams, error) {
	return &connParams{
		url:    mc.URL,
		client: mc.Client,
	}, nil
}

// FixedConnector returns a fixed connector that does not check the connection when calling .Connect().
func FixedConnector(client client.HTTPDoer, u url.URL) Connector {
	return &fixedConnector{
		URL:    u,
		Client: client,
	}
}
