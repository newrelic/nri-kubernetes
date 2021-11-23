package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
)

type connParams struct {
	url    url.URL
	client client.HTTPDoer
}

func defaultConnParamsHTTP(hostURL string) connParams {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	u := url.URL{
		Host:   hostURL,
		Scheme: "http",
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
		Scheme: "https",
	}
	return connParams{u, httpClient}
}

func checkConnectionAPIProxy(apiServer string, nodeName string, tripperBearerToken http.RoundTripper) (*connParams, error) {
	apiURL, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	var conn connParams
	if apiURL.Scheme == "http" {
		conn = defaultConnParamsHTTP(apiURL.Host)
	} else {
		conn = defaultConnParamsHTTPS(apiURL.Host, tripperBearerToken)
	}

	conn.url.Path = path.Join(fmt.Sprintf(apiProxyPath, nodeName))

	if err = checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func checkConnectionHTTP(hostURL string) (*connParams, error) {
	conn := defaultConnParamsHTTP(hostURL)
	if err := checkConnection(conn); err != nil {
		return nil, fmt.Errorf("checking connection via API proxy: %w", err)
	}

	return &conn, nil
}

func checkConnectionHTTPS(hostURL string, tripperBearer http.RoundTripper) (*connParams, error) {
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

func getKubeletPort(kc kubernetes.Interface, nodeName string) (int32, error) {
	//We pay the price of a single call getting a node to avoid asking the user the Kubelet port if different from the standard one
	node, err := kc.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting node %q: %w", nodeName, err)
	}

	return node.Status.DaemonEndpoints.KubeletEndpoint.Port, nil
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
