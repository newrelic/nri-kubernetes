package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/url"
	"path"
	"time"
)

type connParams struct {
	url    url.URL
	client client.HTTPDoer
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
	c, err := GetClientFromRestInterface(kc)
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
		client: c,
	}
	return err, conn
}

// GetClientFromRestInterface is merely an helper to allow using the fake client
var GetClientFromRestInterface = getClientFromRestInterface

func getClientFromRestInterface(kc kubernetes.Interface) (client.HTTPDoer, error) {
	// This could fail then writing tests with fake client. A mock can be used instead.
	secureClient, ok := kc.Discovery().RESTClient().(*rest.RESTClient)
	if !ok {
		return nil, fmt.Errorf("failed to set up a client for connecting to Kubelet through API proxy")
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

func getKubeletPort(kc kubernetes.Interface, nodeName string) (int32, error) {
	//We pay the price of a single call getting a node to avoid asking the user the Kubelet port if different from the standard one

	node, err := kc.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting node %q: %w", nodeName, err)
	}

	return node.Status.DaemonEndpoints.KubeletEndpoint.Port, nil
}
