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
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
	httpSchema   = "http"
	httpsSchema  = "https"
)

type Connector interface {
	connect() (*connParams, error)
}

type defaultConnector struct {
	// TODO: Use a non-sdk logger
	logger             log.Logger
	apiServerHost      string
	kc                 kubernetes.Interface
	tripperBearerToken http.RoundTripper
	config             *config.Mock
}

func (dp *defaultConnector) connect() (*connParams, error) {

	kubeletPort, err := dp.getKubeletPort()
	if err != nil {
		return nil, fmt.Errorf("getting kubelet port: %w", err)
	}

	kubeletSchema := dp.getKubeletSchema(kubeletPort)
	hostURL := net.JoinHostPort(dp.config.NodeIP, fmt.Sprint(kubeletPort))

	conn, err := dp.setupLocalConnection(dp.tripperBearerToken, kubeletSchema, hostURL)
	if err == nil {
		dp.logger.Debugf("connected to Kubelet directly with nodeIP")
		return conn, nil
	}

	dp.logger.Infof("Kubelet connection with nodeIP not working, falling back to API proxy: %v", err)

	conn, err = dp.checkConnectionAPIProxy(dp.apiServerHost, dp.config.NodeName, dp.tripperBearerToken)
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
		// we were not able to infer the schema and the user did not provided it.
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
		dp.logger.Debugf("Schema is unknown since kubeletPort is %d", kubeletPort)
		return ""
	}
}

type connParams struct {
	url    url.URL
	client client.HTTPDoer
}

func (dp *defaultConnector) checkConnectionAPIProxy(apiServer string, nodeName string, tripperBearerToken http.RoundTripper) (*connParams, error) {
	apiURL, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	var conn connParams
	if apiURL.Scheme == httpSchema {
		conn = defaultConnParamsHTTP(apiURL.Host)
	} else {
		conn = defaultConnParamsHTTPS(apiURL.Host, tripperBearerToken)
	}

	conn.url.Path = path.Join(fmt.Sprintf(apiProxyPath, nodeName))

	dp.logger.Debugf("testing kubelet connection with https to host: %s", conn.url.Path)

	if err = checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTP(hostURL string) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection with http to host: %s", hostURL)

	conn := defaultConnParamsHTTP(hostURL)
	if err := checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTPS(hostURL string, tripperBearer http.RoundTripper) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection with https to host: %s", hostURL)

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
		return fmt.Errorf("creating request to: %s. Got error: %s ", conn.url.String(), err)
	}

	resp, err := conn.client.Do(r)
	if err != nil {
		return fmt.Errorf("connecting to: %s. Got error: %s ", conn.url.String(), err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error calling endpoint %s. Got status code: %d", conn.url.String(), resp.StatusCode)
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

func defaultConnParamsHTTPS(hostURL string, tripperBearerToken http.RoundTripper) connParams {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	httpClient.Transport = tripperBearerToken

	u := url.URL{
		Host:   hostURL,
		Scheme: httpsSchema,
	}
	return connParams{u, httpClient}
}

type MockConnector struct {
	URL    url.URL
	Client client.HTTPDoer
	Err    error
}

func (mc MockConnector) connect() (*connParams, error) {
	return &connParams{
		url:    mc.URL,
		client: mc.Client,
	}, mc.Err
}
