package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const (
	bearerToken    = "12345"
	prometheusPath = "/metrics"
)

func Test_Client_scrape_http_endpoint(t *testing.T) {
	t.Parallel()

	server := testHTTPServer(t)

	endpoints := []config.Endpoint{{URL: server.URL}}

	cpClient, err := client.New(getTestConnector(server, endpoints))
	assert.NoError(t, err)

	t.Run("return_ok_when_succeed", func(t *testing.T) {
		r, err := cpClient.Get(prometheusPath)
		require.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)
	})
}

func Test_Client_tries_endpoints_list(t *testing.T) {
	t.Parallel()

	server := testHTTPServer(t)
	requestCount := 0
	failServer := testHTTPServerFail(t, &requestCount)

	endpoints := []config.Endpoint{
		{
			URL: failServer.URL,
		},
		{
			URL:  failServer.URL,
			Auth: &config.Auth{Type: "bearer"},
		},
		{
			URL: server.URL,
		},
	}

	cpClient, err := client.New(getTestConnector(server, endpoints))
	assert.NoError(t, err)

	r, err := cpClient.Get(prometheusPath)
	require.NoError(t, err)
	assert.Equal(t, r.StatusCode, http.StatusOK)
	assert.Equal(t, 2, requestCount)
}

func Test_Client_scrape_https_endpoint(t *testing.T) {
	t.Parallel()

	server := testHTTPSServer(t)
	endpoints := []config.Endpoint{
		{
			URL:                server.URL,
			InsecureSkipVerify: true,
		},
	}

	cpClient, err := client.New(getTestConnector(server, endpoints))
	assert.NoError(t, err)

	r, err := cpClient.Get(prometheusPath)
	assert.NoError(t, err)
	assert.Equal(t, r.StatusCode, http.StatusOK)
}

func Test_Client_scrape_https_endpoint_with_bearer_token_auth(t *testing.T) {
	t.Parallel()

	server, h := testHTTPSServerBearer(t)
	endpoints := []config.Endpoint{
		{
			URL:                server.URL,
			InsecureSkipVerify: true,
			Auth: &config.Auth{
				Type: "bearer",
			},
		},
	}

	cpClient, err := client.New(getTestConnector(server, endpoints))
	assert.NoError(t, err)

	r, err := cpClient.Get(prometheusPath)
	assert.NoError(t, err)
	assert.Equal(t, r.StatusCode, http.StatusOK)
	assert.Contains(t, h.Get("Authorization"), bearerToken)
}

func testHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}

func testHTTPServerFail(t *testing.T, requestCount *int) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		*requestCount++
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}

func testHTTPSServer(t *testing.T) *httptest.Server {
	t.Helper()

	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}

func testHTTPSServerBearer(t *testing.T) (*httptest.Server, *http.Header) {
	t.Helper()

	h := &http.Header{}

	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*h = r.Header
		if !strings.Contains(r.Header.Get("Authorization"), bearerToken) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer, h
}

func getTestConnector(s *httptest.Server, endpoints []config.Endpoint) client.Connector {
	u, _ := url.Parse(s.URL)

	c := fake.NewSimpleClientset()

	inClusterConfig := &rest.Config{
		Host:        fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	connector, _ := client.DefaultConnector(endpoints, c, inClusterConfig, log.NewStdErr(true))

	return connector
}
