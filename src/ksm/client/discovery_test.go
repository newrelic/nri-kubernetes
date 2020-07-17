package client

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"fmt"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"
)

func fakeLookupSRV(service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	return "cname", []*net.SRV{{Port: 11223}}, nil
}

func emptyLookupSRV(service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	return "cname", []*net.SRV{}, nil
}

func failingLookupSRV(service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	return "cname", nil, fmt.Errorf("patapum")
}

func mockResponseHandler(mockResponse io.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, mockResponse) // nolint: errcheck
	}
}

const timeout = time.Second

var logger = logrus.StandardLogger()

// Testing Discover() method
func TestDiscover_portThroughDNS(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)
	// And an Discoverer implementation
	d := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the KSM Service is returned
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	// And the nodeIP is correctly returned
	assert.Equal(t, "6.7.8.9", ksmClient.(*ksm).nodeIP)
}

func TestDiscover_portThroughDNSAndGuessedNodeIPFromMultiplePods(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "6.7.8.9"}},
			{Status: v1.PodStatus{HostIP: "162.178.1.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
		}}, nil)

	// and an Discoverer implementation
	d := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}
	// When retrieving the KSM client with no port named 'http-metrics'
	ksmClient, err := d.Discover(timeout)

	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the KSM Service is returned
	assert.Equal(t, fmt.Sprintf("%s:%v", ksmQualifiedName, 11223), ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	// And the nodeIP is correctly returned
	assert.Equal(t, "162.178.1.1", ksmClient.(*ksm).nodeIP)
}
func TestDiscover_metricsPortThroughAPIWhenDNSEmptyResponse(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{Items: []v1.Service{{
			Spec: v1.ServiceSpec{
				ClusterIP: "1.2.3.4",
				Ports: []v1.ServicePort{{
					Name: ksmPortName,
					Port: 8888,
				}},
			},
		},
		}}, nil)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// and an Discoverer implementation whose DNS returns empty response
	d := discoverer{
		lookupSRV: emptyLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When discovering the KSM client
	ksmClient, err := d.Discover(timeout)

	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the KSM Service is returned
	assert.Equal(t, "1.2.3.4:8888", ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	// And the nodeIP is correctly returned
	assert.Equal(t, "6.7.8.9", ksmClient.(*ksm).nodeIP)
}

func TestDiscover_metricsPortThroughAPIWhenDNSError(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{Items: []v1.Service{{
			Spec: v1.ServiceSpec{
				ClusterIP: "1.2.3.4",
				Ports: []v1.ServicePort{{
					Name: ksmPortName,
					Port: 8888,
				}},
			},
		},
		}}, nil)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// and an Discoverer implementation whose DNS returns an error
	d := discoverer{
		lookupSRV: failingLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the discovered host:port of the KSM Service is returned
	assert.Equal(t, "1.2.3.4:8888", ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	// And the nodeIP is correctly returned
	assert.Equal(t, "6.7.8.9", ksmClient.(*ksm).nodeIP)
}

func TestDiscover_guessedTCPPortThroughAPIWhenDNSEmptyResponse(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{Items: []v1.Service{{
			Spec: v1.ServiceSpec{
				ClusterIP: "11.22.33.44",
				Ports: []v1.ServicePort{{
					Name:     "SomeCoolPort",
					Protocol: "UDP",
					Port:     1234,
				}, {
					Name:     "ThisPortShouldWork",
					Protocol: "TCP",
					Port:     8081,
				}},
			}}}}, nil)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// and an Discoverer implementation whose DNS returns empty response
	d := discoverer{
		lookupSRV: emptyLookupSRV,
		apiClient: c,
		logger:    logger,
	}
	// When retrieving the KSM client with no port named 'http-metrics'
	ksmClient, err := d.Discover(timeout)

	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And the first TCP host:port of the KSM Service is returned
	assert.Equal(t, "11.22.33.44:8081", ksmClient.(*ksm).endpoint.Host)
	assert.Equal(t, "http", ksmClient.(*ksm).endpoint.Scheme)
	// And the nodeIP is correctly returned
	assert.Equal(t, "6.7.8.9", ksmClient.(*ksm).nodeIP)
}

func TestDiscover_errorRetrievingPortWhenDNSAndAPIResponsesEmpty(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	// And FindServicesByLabel returns empty list
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{}, nil)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// and an Discoverer implementation whose DNS returns empty response
	d := discoverer{
		lookupSRV: emptyLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call returns the error
	assert.EqualError(t, err, "failed to discover kube-state-metrics endpoint, got error: no services found by any of labels k8s-app, app with value kube-state-metrics")

	// And the KSM client is not returned
	assert.Nil(t, ksmClient)
}

func TestDiscover_errorRetrievingPortWhenDNSAndAPIErrors(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	// And FindServicesByLabel returns error
	c.On("FindServicesByLabel", mock.Anything, mock.Anything).
		Return(&v1.ServiceList{}, errors.New("failure"))
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{{
			Status: v1.PodStatus{HostIP: "6.7.8.9"},
		}}}, nil)

	// and an Discoverer implementation whose DNS returns an error
	d := discoverer{
		lookupSRV: failingLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call returns the error
	assert.EqualError(t, err, "failed to discover kube-state-metrics endpoint, got error: failure")

	// And the KSM client is not returned
	assert.Nil(t, ksmClient)
}
func TestDiscover_errorRetrievingNodeIPWhenPodListEmpty(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	// And FindPodsByLabel returns empty list
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{}, nil)
	// And an Discoverer implementation
	d := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call returns the error
	assert.EqualError(t, err, "failed to discover nodeIP with kube-state-metrics, got error: no pods found by any of labels k8s-app, app with value kube-state-metrics")

	// And the KSM client is not returned
	assert.Nil(t, ksmClient)
}

func TestDiscover_errorRetrievingNodeIPWhenErrorFindingPod(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	// And FindPodsByLabel returns error
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{}, errors.New("failure"))
	// And an Discoverer implementation
	d := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	// When retrieving the KSM client
	ksmClient, err := d.Discover(timeout)
	// The call returns the error
	assert.EqualError(t, err, "failed to discover nodeIP with kube-state-metrics, got error: failure")

	// And the KSM client is not returned
	assert.Nil(t, ksmClient)
}

func TestNodeIPForDiscoverer_Error(t *testing.T) {
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "6.7.8.9"}},
		}}, errors.New("no label"))
	d := discoverer{
		lookupSRV: fakeLookupSRV,
		apiClient: c,
		logger:    logger,
	}

	nodeIP, err := d.nodeIP()

	assert.EqualError(t, err, "no label")
	assert.Equal(t, "", nodeIP)
}

// Testing NodeIP() method
func TestNodeIP(t *testing.T) {
	// Given a ksm struct initialized
	var c = ksm{
		nodeIP:     "1.2.3.4",
		endpoint:   url.URL{},
		httpClient: http.DefaultClient,
		logger:     logger,
	}
	var cl = &c
	// When retrieving node IP
	nodeIP := cl.NodeIP()
	// The call works correctly
	assert.Equal(t, "1.2.3.4", nodeIP)
}

// Testing Do() method
func TestDo(t *testing.T) {
	r := strings.NewReader("Foo")
	s := httptest.NewServer(mockResponseHandler(r))
	endpoint, err := url.Parse(s.URL)
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	var c = &ksm{
		nodeIP:     "1.2.3.4",
		endpoint:   *endpoint,
		httpClient: s.Client(),
		logger:     logger,
	}

	// When retrieving http response
	resp, err := c.Do("GET", "foo")

	// The call works correctly
	assert.NoError(t, err)
	// The request was created with updated path for URL
	assert.Equal(t, fmt.Sprintf("%s/foo", s.URL), resp.Request.URL.String())
	// Accept Header was added to the request
	assert.Equal(t, prometheus.AcceptHeader, resp.Request.Header.Get("Accept"))
	// Correct http method was used
	assert.Equal(t, "GET", resp.Request.Method)
}

func TestDo_error(t *testing.T) {
	client := &ksm{
		nodeIP:     "",
		endpoint:   url.URL{},
		httpClient: http.DefaultClient,
		logger:     logger,
	}

	// When retrieving http response
	resp, err := client.Do("", "")

	// The call returns error
	assert.NotNil(t, err)
	// The response was not created
	assert.Nil(t, resp)
}
