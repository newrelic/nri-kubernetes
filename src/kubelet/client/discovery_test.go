package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

const (
	timeout               = time.Second
	fakeDiscoveredAPIHost = "111.111.11.1"
	restConfigAPIHost     = "https://111.111.11.1"
)

var logger = logrus.StandardLogger()

// Kubernetes API client mocks

func failingClientMock() *client.MockedKubernetes {
	c := new(client.MockedKubernetes)
	c.On("Config").Return(&rest.Config{})
	c.On("SecureHTTPClient", mock.Anything).Return(&http.Client{}, nil)
	c.On("FindNode", mock.Anything).Return(nil, errors.New("FindNode should not be invoked"))
	return c
}

// creates a mocked Kubernetes API client
func mockedClient() *client.MockedKubernetes {
	c := new(client.MockedKubernetes)
	c.On("Config").Return(&rest.Config{BearerToken: "d34db33f", Host: restConfigAPIHost})
	c.On("SecureHTTPClient", mock.Anything).Return(&http.Client{}, nil)
	return c
}

// sets the result of the FindNode function in the Kubernetes API Client
func onFindNode(c *client.MockedKubernetes, nodeName, internalIP string, kubeletPort int) {
	c.On("FindNode", nodeName).
		Return(&v1.Node{
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{
						Type:    "InternalIP",
						Address: internalIP,
					},
				},
				DaemonEndpoints: v1.NodeDaemonEndpoints{
					KubeletEndpoint: v1.DaemonEndpoint{
						Port: int32(kubeletPort),
					},
				},
			},
		}, nil)
}

// Connection checker mocks

func allOkConnectionChecker(_ *http.Client, _ url.URL, _, _ string) error {
	return nil
}

func failOnInsecureConnection(_ *http.Client, URL url.URL, _, _ string) error {
	if URL.Scheme != "https" {
		return fmt.Errorf("the connection can't be established")
	}
	return nil
}

func onlyAPIConnectionChecker(_ *http.Client, URL url.URL, _, _ string) error {
	if URL.Host == fakeDiscoveredAPIHost {
		return nil
	}
	return fmt.Errorf("the connection can't be established")
}

func mockStatusCodeHandler(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}
}

func TestDiscoverHTTP_DefaultInsecurePort(t *testing.T) {
	c := mockedClient()
	onFindNode(c, defaultNodeName, "1.2.3.4", defaultInsecureKubeletPort)

	d := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: allOkConnectionChecker,
		logger:      logger,
	}

	// When retrieving the Kubelet URL
	kclient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the Kubelet is returned
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:10255", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "http", kclient.(*kubelet).endpoint.Scheme)
}

func TestDiscoverHTTPS_DefaultSecurePort(t *testing.T) {
	c := mockedClient()
	onFindNode(c, defaultNodeName, "1.2.3.4", defaultSecureKubeletPort)

	d := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: allOkConnectionChecker,
		logger:      logger,
	}

	// When retrieving the Kubelet URL
	kclient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the Kubelet is returned
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:10250", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "https", kclient.(*kubelet).endpoint.Scheme)
}

func TestDiscoverHTTP_CheckingConnection(t *testing.T) {
	c := mockedClient()
	// Whose Kubelet has an endpoint in a non-default port
	onFindNode(c, defaultNodeName, "1.2.3.4", 55332)

	d := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: allOkConnectionChecker,
		logger:      logger,
	}

	// When retrieving the Kubelet URL
	kclient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the Kubelet is returned
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:55332", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "http", kclient.(*kubelet).endpoint.Scheme)
}

func TestDiscoverHTTPS_CheckingConnection(t *testing.T) {
	c := mockedClient()
	// Whose Kubelet has an endpoint in a non-default port
	onFindNode(c, defaultNodeName, "1.2.3.4", 55332)

	// and an Discoverer implementation whose connection check connection fails because it is a secure connection
	d := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: failOnInsecureConnection,
		logger:      logger,
	}

	// When retrieving the Kubelet URL
	kclient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the Kubelet is returned
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, "1.2.3.4:55332", kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "https", kclient.(*kubelet).endpoint.Scheme)
}

func TestDiscoverHTTPS_ApiConnection(t *testing.T) {
	c := mockedClient()
	// Whose Kubelet has an endpoint in a non-default port
	onFindNode(c, defaultNodeName, "1.2.3.4", 55332)

	// and an Discoverer implementation whose connection check connection fails because it is a secure connection
	d := discoverer{
		nodeName:    defaultNodeName,
		apiClient:   c,
		connChecker: onlyAPIConnectionChecker,
		logger:      logger,
	}

	// When retrieving the Kubelet URL
	kclient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the Kubelet is returned
	assert.Equal(t, "1.2.3.4", kclient.NodeIP())
	assert.Equal(t, fakeDiscoveredAPIHost, kclient.(*kubelet).endpoint.Host)
	assert.Equal(t, "https", kclient.(*kubelet).endpoint.Scheme)
}

func TestDiscover_NodeNotFoundError(t *testing.T) {
	c := mockedClient()

	// That doesn't find the node by Name
	c.On("FindNode", defaultNodeName).Return(&v1.Node{}, fmt.Errorf("Node not found"))

	d := discoverer{
		nodeName:  defaultNodeName,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the Kubelet URL
	_, err := d.Discover(timeout)
	// The system returns an error
	assert.NotNil(t, err, "should return error")
}

func TestDo_HTTP(t *testing.T) {
	s := httptest.NewServer(mockStatusCodeHandler(http.StatusOK))
	defer s.Close()

	endpoint, err := url.Parse(s.URL)
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	var c = &kubelet{
		nodeIP:     "1.2.3.4",
		config:     rest.Config{BearerToken: "Foo"},
		nodeName:   "nodeFoo",
		endpoint:   *endpoint,
		httpClient: s.Client(),
		logger:     logger,
	}

	expectedCalledURL := fmt.Sprintf("%s/foo", s.URL)

	resp, err := c.Do("GET", "foo")

	assert.NoError(t, err)
	assert.Equal(t, expectedCalledURL, resp.Request.URL.String())
	assert.Equal(t, "", resp.Request.Header.Get("Authorization"))
	assert.Equal(t, "GET", resp.Request.Method)
	assert.Equal(t, s.URL, endpoint.String())
}

func TestDo_HTTPS(t *testing.T) {
	s := httptest.NewTLSServer(mockStatusCodeHandler(http.StatusOK))
	defer s.Close()

	endpoint, err := url.Parse(s.URL)
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	var c = &kubelet{
		nodeIP:     "1.2.3.4",
		config:     rest.Config{BearerToken: "Foo"},
		nodeName:   "nodeFoo",
		endpoint:   *endpoint,
		httpClient: s.Client(),
		logger:     logger,
	}

	expectedCalledURL := fmt.Sprintf("%s/foo", s.URL)

	resp, err := c.Do("GET", "foo")

	assert.NoError(t, err)
	assert.Equal(t, expectedCalledURL, resp.Request.URL.String())
	assert.Equal(t, fmt.Sprintf("Bearer %s", c.config.BearerToken), resp.Request.Header.Get("Authorization"))
	assert.Equal(t, "GET", resp.Request.Method)
	assert.Equal(t, s.URL, endpoint.String())
}

func TestCheckCall(t *testing.T) {
	s := httptest.NewServer(mockStatusCodeHandler(http.StatusOK))
	defer s.Close()

	endpoint, err := url.Parse(s.URL)
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	err = checkCall(s.Client(), *endpoint, "foo", "foo token")
	assert.NoError(t, err)
}

func TestCheckCall_ErrorNotSuccessStatusCode(t *testing.T) {
	s := httptest.NewTLSServer(mockStatusCodeHandler(http.StatusBadRequest))
	defer s.Close()

	endpoint, err := url.Parse(s.URL)
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	expectedCalledURL := fmt.Sprintf("%s/foo", s.URL)

	err = checkCall(s.Client(), *endpoint, "foo", "foo token")
	assert.EqualError(t, err, fmt.Sprintf("error calling endpoint %s. Got status code: %d", expectedCalledURL, http.StatusBadRequest))
}

// Error comes from http Do method from RoundTripper interface.
// Empty url is passed to Do method and error unsupported protocol scheme is received
func TestCheckCall_ErrorConnecting(t *testing.T) {
	err := checkCall(http.DefaultClient, url.URL{}, "", "")
	assert.Error(t, err)
}
