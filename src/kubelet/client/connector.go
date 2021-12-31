package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/src/common"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
	httpScheme   = "http"
	httpsScheme  = "https"

	defaultHTTPKubeletPort  = 10255
	defaultHTTPSKubeletPort = 10250
	defaultTimeout          = time.Millisecond * 5000
)

var ErrNoConnection = errors.New("no suitable connection methods found")

// Connector provides an interface to retrieve connParams to connect to a Kubelet instance.
type Connector interface {
	Connect() (*Client, error)
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
// Locally it adds the bearer token to the request if protocol=https sing transport.NewBearerAuthRoundTripper,
// on the other hand passing through api-proxy, authentication is managed by the kubernetes client itself.
// Notice that we cannot use the as well rest.TransportFor to connect locally since the certificate sent by kubelet,
// cannot be verified in the same way we do for the apiServer.
func (dp *defaultConnector) Connect() (*Client, error) {
	kubeletPort, err := dp.getPort()
	if err != nil {
		return nil, fmt.Errorf("getting kubelet port: %w", err)
	}

	kubeletScheme := dp.schemeFor(kubeletPort)
	hostURL := net.JoinHostPort(dp.config.NodeIP, fmt.Sprint(kubeletPort))

	dp.logger.Infof("Trying to connect to kubelet locally with scheme=%q hostURL=%q", kubeletScheme, hostURL)
	conn, err := dp.checkLocalConnection(tripperWithBearerToken(dp.inClusterConfig.BearerToken), kubeletScheme, hostURL)
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

	conn, err = dp.probeAPIServerProxy(dp.inClusterConfig.Host, dp.config.NodeName, tripperAPI)
	if err != nil {
		return nil, fmt.Errorf("creating connection parameters for API proxy: %w", err)
	}

	return conn, nil
}

func (dp *defaultConnector) checkLocalConnection(tripperWithBearerToken http.RoundTripper, scheme string, hostURL string) (*Client, error) {
	dp.logger.Debugf("Attempting to connect to kubelet using node IP")

	switch scheme {
	case httpScheme:
		dp.logger.Debugf("Testing kubelet connection over http to %s", hostURL)

		cli, err := dp.probeLocalHTTP(hostURL)
		if err != nil {
			dp.logger.Debugf("Error probing Kubelet over http: %v", err)
		}

		return cli, nil

	case httpsScheme:
		dp.logger.Debugf("Testing kubelet connection over https to %s", hostURL)

		cli, err := dp.probeLocalHTTPS(hostURL, tripperWithBearerToken)
		if err != nil {
			dp.logger.Debugf("Error probing Kubelet over https: %v", err)
			return nil, err
		}

		return cli, nil

	default:
		dp.logger.Debugf("Testing kubelet connection over http to %s", hostURL)
		cli, err := dp.probeLocalHTTP(hostURL)
		if err == nil {
			return cli, nil
		}
		dp.logger.Debugf("Error probing Kubelet over http: %v", err)

		dp.logger.Debugf("Testing kubelet connection over https to %s", hostURL)
		cli, err = dp.probeLocalHTTPS(hostURL, tripperWithBearerToken)
		if err == nil {
			return cli, nil
		}
		dp.logger.Debugf("Error probing Kubelet over https: %v", err)
	}

	return nil, ErrNoConnection
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

func (dp *defaultConnector) probeAPIServerProxy(apiServer string, nodeName string, tripperAPIproxy http.RoundTripper) (*Client, error) {
	apiURL, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	apiServerPath := path.Join(fmt.Sprintf(apiProxyPath, nodeName))
	dp.logger.Debugf("Testing kubelet connection through API proxy: %s%s", apiURL.Host, apiServerPath)

	cli := NewClient(
		url.URL{
			Host:   apiURL.Host,
			Scheme: apiURL.Scheme,
			Path:   apiServerPath,
		},
		&http.Client{
			Timeout:   defaultTimeout,
			Transport: tripperAPIproxy,
		},
	)

	if err := cli.Probe(); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return cli, nil
}

func (dp *defaultConnector) probeLocalHTTP(hostURL string) (*Client, error) {
	cli := NewClient(
		url.URL{
			Host:   hostURL,
			Scheme: httpScheme,
		},
		http.DefaultClient,
	)

	if err := cli.Probe(); err != nil {
		return nil, err
	}

	return cli, nil
}

func (dp *defaultConnector) probeLocalHTTPS(hostURL string, tripperBearer http.RoundTripper) (*Client, error) {
	cli := NewClient(
		url.URL{
			Host:   hostURL,
			Scheme: httpsScheme,
		},
		&http.Client{
			Timeout:   defaultTimeout,
			Transport: tripperBearer,
		},
	)

	if err := cli.Probe(); err != nil {
		return nil, err
	}

	return cli, nil
}

// connParams holds a client and a base URL for the
type connParams struct {
	url    url.URL
	client common.HTTPDoer
}

func (c connParams) Client() common.HTTPClient {
	return common.NewHTTP(c.client)
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
