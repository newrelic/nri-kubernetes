package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	apiProxyPath = "/api/v1/nodes/%s/proxy/"
)

type connParams struct {
	url    url.URL
	client client.HTTPDoer
}

func defaultConnParams(tripper http.RoundTripper, hostURL string) connParams {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}
	httpClient.Transport = tripper
	u := url.URL{
		Host: hostURL,
	}
	return connParams{u, httpClient}
}

func connectionAPIProxy(tripper http.RoundTripper, apiServer string, nodeName string) (connParams, error) {
	apiURL, err := url.Parse(apiServer)
	if err != nil {
		err = fmt.Errorf("parsing kubernetes api url from in cluster config: %w", err)
	}

	conn := defaultConnParams(tripper, apiURL.Host)
	conn.url.Scheme = apiURL.Scheme
	conn.url.Path = fmt.Sprintf(apiProxyPath, nodeName)

	return conn, nil
}

func checkCall(conn connParams) error {
	conn.url.Path = path.Join(conn.url.Path, healthzPath)

	r, err := http.NewRequest(http.MethodGet, conn.url.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request to: %s. Got error: %s ", conn.url.String(), err)
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

func getKubeletPort(kc kubernetes.Interface, nodeName string) (int32, error) {
	//We pay the price of a single call getting a node to avoid asking the user the Kubelet port if different from the standard one

	node, err := kc.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting node %q: %w", nodeName, err)
	}

	return node.Status.DaemonEndpoints.KubeletEndpoint.Port, nil
}
