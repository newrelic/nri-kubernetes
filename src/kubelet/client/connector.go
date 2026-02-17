package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
	httpScheme   = "http"
	httpsScheme  = "https"
)

var errBadStatusCode = fmt.Errorf("non-200 status code")

// Connector provides an interface to retrieve connParams to connect to a Kubelet instance.
type Connector interface {
	Connect() (*connParams, error)
}

type defaultConnector struct {
	logger          *log.Logger
	kc              kubernetes.Interface
	inClusterConfig *rest.Config
	config          *config.Config
}

// DefaultConnector returns a defaultConnector that checks connection against local kubelet and api proxy.
func DefaultConnector(kc kubernetes.Interface, config *config.Config, inClusterConfig *rest.Config, logger *log.Logger) Connector {
	return &defaultConnector{
		logger:          logger,
		inClusterConfig: inClusterConfig,
		kc:              kc,
		config:          config,
	}
}

// Connect probes the kubelet connection locally and then through the apiServer proxy.
// It tries to infer the scheme and the port found in node status DaemonEndpoints, if needed the user can set them up.
// Locally it adds the bearer token from the file to the request if protocol=https sing transport.NewBearerAuthWithRefreshRoundTripper,
// on the other hand passing through api-proxy, authentication is managed by the kubernetes client itself.
// Notice that we cannot use the as well rest.TransportFor to connect locally since the certificate sent by kubelet,
// cannot be verified in the same way we do for the apiServer.
//
// If InitTimeout is configured, Connect will retry connection attempts until successful or timeout is reached.
// This is useful in environments like EKS where kubelet certificates may take 1-2 minutes to provision after node startup.
func (dp *defaultConnector) Connect() (*connParams, error) {
	// Get kubelet port and scheme once before retry loop
	kubeletPort, err := dp.getPort()
	if err != nil {
		return nil, fmt.Errorf("getting kubelet port: %w", err)
	}

	kubeletScheme := dp.schemeFor(kubeletPort)

	// If InitTimeout is 0, use legacy behavior (no retries)
	if dp.config.InitTimeout == 0 {
		return dp.tryConnect(kubeletPort, kubeletScheme)
	}

	// Retry logic with timeout
	return dp.connectWithRetry(kubeletPort, kubeletScheme)
}

// connectWithRetry attempts to connect to kubelet with retries until successful or timeout is reached.
func (dp *defaultConnector) connectWithRetry(kubeletPort int32, kubeletScheme string) (*connParams, error) {
	start := time.Now()
	attempt := 0
	var lastErr error

	dp.logger.Infof("Attempting to connect to kubelet with retry timeout=%s backoff=%s",
		dp.config.InitTimeout, dp.config.InitBackoff)

	for {
		attempt++
		elapsed := time.Since(start)

		// Try to connect
		conn, err := dp.tryConnect(kubeletPort, kubeletScheme)
		if err == nil {
			dp.logger.Infof("Successfully connected to kubelet on attempt %d after %s", attempt, elapsed)
			return conn, nil
		}

		lastErr = err

		// Check if we've exceeded timeout
		if elapsed >= dp.config.InitTimeout {
			return nil, fmt.Errorf("failed to connect to kubelet after %d attempts over %s (timeout: %s): %w",
				attempt, elapsed, dp.config.InitTimeout, lastErr)
		}

		// Calculate remaining time and adjust backoff if needed
		remainingTime := dp.config.InitTimeout - elapsed
		backoff := dp.config.InitBackoff
		if backoff > remainingTime {
			backoff = remainingTime
		}

		// Log retry information
		dp.logger.Infof("Kubelet connection attempt %d failed: %v. Retrying in %s (elapsed: %s/%s)",
			attempt, err, backoff, elapsed, dp.config.InitTimeout)

		// Wait before next attempt
		time.Sleep(backoff)
	}
}

// tryConnect performs a single connection attempt to kubelet (local and API proxy fallback).
// This is the existing Connect() logic extracted into a separate method.
func (dp *defaultConnector) tryConnect(kubeletPort int32, kubeletScheme string) (*connParams, error) {
	hostURL := net.JoinHostPort(dp.config.NodeIP, fmt.Sprint(kubeletPort))

	dp.logger.Infof("Trying to connect to kubelet locally with scheme=%q hostURL=%q", kubeletScheme, hostURL)
	trip, err := tripperWithBearerTokenAndRefresh(dp.inClusterConfig.BearerTokenFile)
	if err != nil {
		return nil, fmt.Errorf("creating tripper connecting to kubelet through nodeIP: %w", err)
	}

	conn, err := dp.checkLocalConnection(trip, kubeletScheme, hostURL)
	if err == nil {
		dp.logger.Infof("Connected to Kubelet through nodeIP with scheme=%q hostURL=%q", kubeletScheme, hostURL)
		return conn, nil
	}
	dp.logger.Infof("Kubelet not reachable locally with scheme=%q hostURL=%q: %v", kubeletScheme, hostURL, err)

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

func (dp *defaultConnector) checkLocalConnection(tripperWithBearerTokenRefreshing http.RoundTripper, scheme string, hostURL string) (*connParams, error) {
	dp.logger.Debugf("connecting to kubelet directly with nodeIP")
	var err error
	var conn *connParams

	switch scheme {
	case httpScheme:
		if conn, err = dp.checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}
	case httpsScheme:
		if conn, err = dp.checkConnectionHTTPS(hostURL, tripperWithBearerTokenRefreshing); err == nil {
			return conn, nil
		}
	default:
		dp.logger.Infof("Checking both HTTP and HTTPS since the scheme was not detected automatically, " +
			"you can set set kubelet.scheme to avoid this behaviour")

		if conn, err = dp.checkConnectionHTTPS(hostURL, tripperWithBearerTokenRefreshing); err == nil {
			return conn, nil
		}

		if conn, err = dp.checkConnectionHTTP(hostURL); err == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("no connection succeeded through localhost: %w", err)
}

func (dp *defaultConnector) getPort() (int32, error) {
	if dp.config.Kubelet.Port != 0 {
		dp.logger.Debugf("Setting Port %d as specified by user config", dp.config.Kubelet.Port)
		return dp.config.Kubelet.Port, nil
	}

	// We pay the price of a single call getting a node to avoid asking the user the Kubelet port.
	node, err := dp.kc.CoreV1().Nodes().Get(context.Background(), dp.config.NodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting node %q: %w", dp.config.NodeName, err)
	}

	port := node.Status.DaemonEndpoints.KubeletEndpoint.Port
	dp.logger.Debugf("Setting Port %d as found in status condition", port)

	return port, nil
}

func (dp *defaultConnector) getTestConnectionEndpoint() string {
	if dp.config.TestConnectionEndpoint != "" {
		return dp.config.TestConnectionEndpoint
	}
	return healthzPath
}

func (dp *defaultConnector) schemeFor(kubeletPort int32) string {
	if dp.config.Kubelet.Scheme != "" {
		dp.logger.Debugf("Setting Kubelet Endpoint Scheme %s as specified by user config", dp.config.Kubelet.Scheme)
		return dp.config.Kubelet.Scheme
	}

	switch kubeletPort {
	case defaultHTTPKubeletPort:
		dp.logger.Debugf("Setting Kubelet Endpoint Scheme http since kubeletPort is %d", kubeletPort)
		return httpScheme
	case defaultHTTPSKubeletPort:
		dp.logger.Debugf("Setting Kubelet Endpoint Scheme https since kubeletPort is %d", kubeletPort)
		return httpsScheme
	default:
		dp.logger.Infof("Cannot automatically figure out scheme from non-standard port %d, please set kubelet.scheme in the config file.", kubeletPort)
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
			Timeout:   dp.config.Kubelet.Timeout,
			Transport: tripperAPIproxy,
		},
		url: url.URL{
			Host:   apiURL.Host,
			Scheme: apiURL.Scheme,
			Path:   path.Join(fmt.Sprintf(apiProxyPath, nodeName)),
		},
	}

	dp.logger.Debugf("Testing kubelet connection through API proxy: %s%s", apiURL.Host, conn.url.Path)

	if err = checkConnection(conn, dp.getTestConnectionEndpoint()); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTP(hostURL string) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection over plain http to %s", hostURL)

	conn := dp.defaultConnParamsHTTP(hostURL)
	if err := checkConnection(conn, dp.getTestConnectionEndpoint()); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func (dp *defaultConnector) checkConnectionHTTPS(hostURL string, tripperBearerRefreshing http.RoundTripper) (*connParams, error) {
	dp.logger.Debugf("testing kubelet connection over https to %s", hostURL)

	conn := dp.defaultConnParamsHTTPS(hostURL, tripperBearerRefreshing)
	if err := checkConnection(conn, dp.getTestConnectionEndpoint()); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func checkConnection(connParams connParams, endpoint string) error {
	connParams.url.Path = path.Join(connParams.url.Path, endpoint)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, connParams.url.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request to %q: %w", connParams.url.String(), err)
	}

	resp, err := connParams.client.Do(request)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", connParams.url.String(), err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("calling %s got %w: %d", connParams.url.String(), errBadStatusCode, resp.StatusCode)
	}

	return nil
}

func tripperWithBearerTokenAndRefresh(tokenFile string) (http.RoundTripper, error) {
	// Here we're using the default http.Transport configuration, but with a modified TLS config.
	// The DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig.InsecureSkipVerify = true
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// Use the default kubernetes Bearer token authentication RoundTripper
	tripperWithBearerRefreshing, err := transport.NewBearerAuthWithRefreshRoundTripper("", tokenFile, t)
	if err != nil {
		return nil, fmt.Errorf("creating bearerAuthWithRefreshRoundTripper: %w", err)
	}

	return tripperWithBearerRefreshing, nil
}

func (dp *defaultConnector) defaultConnParamsHTTP(hostURL string) connParams {
	httpClient := &http.Client{
		Timeout: dp.config.Kubelet.Timeout,
	}

	u := url.URL{
		Host:   hostURL,
		Scheme: httpScheme,
	}
	return connParams{u, httpClient}
}

func (dp *defaultConnector) defaultConnParamsHTTPS(hostURL string, tripper http.RoundTripper) connParams {
	httpClient := &http.Client{
		Timeout: dp.config.Kubelet.Timeout,
	}

	httpClient.Transport = tripper

	u := url.URL{
		Host:   hostURL,
		Scheme: httpsScheme,
	}
	return connParams{u, httpClient}
}

type fixedConnector struct {
	URL    url.URL
	Client client.HTTPDoer
}

// Connect return connParams without probing any endpoint.
func (mc *fixedConnector) Connect() (*connParams, error) {
	return &connParams{
		url:    mc.URL,
		client: mc.Client,
	}, nil
}

// StaticConnector returns a fixed connector that does not check the connection when calling .Connect().
func StaticConnector(client client.HTTPDoer, u url.URL) Connector {
	return &fixedConnector{
		URL:    u,
		Client: client,
	}
}
