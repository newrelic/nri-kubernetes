package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const (
	bearerToken    = "12345"
	prometheusPath = "/metrics"
	timeoutPath    = "/timeout"
)

func Test_Client_scrape_http_endpoint(t *testing.T) {
	server := testHTTPServer(t)

	cpClient, err := client.New(getTestConfig(server))
	assert.NoError(t, err)

	t.Run("return_ok_when_succeed", func(t *testing.T) {
		r, err := cpClient.Get(prometheusPath)
		require.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)
	})
	t.Run("error_on_timeout", func(t *testing.T) {
		r, err := cpClient.Get(timeoutPath)
		require.Error(t, err)
		assert.Nil(t, r)
	})
}

func Test_Client_scrape_https_endpoint(t *testing.T) {
	server, h := testHTTPSServer(t)

	cpClient, err := client.New(getTestConfig(server))
	assert.NoError(t, err)

	t.Run("secured_endpoint", func(t *testing.T) {
		r, err := cpClient.Get("/metrics")
		assert.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)
		assert.Contains(t, h.Get("Authorization"), bearerToken)
	})
}

func testHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == timeoutPath {
			time.Sleep(time.Second)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	return testServer
}

func testHTTPSServer(t *testing.T) (*httptest.Server, *http.Header) {
	t.Helper()
	h := &http.Header{}

	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*h = r.Header
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	return testServer, h
}

func getTestConfig(s *httptest.Server) client.Config {
	u, _ := url.Parse(s.URL)

	c := fake.NewSimpleClientset()

	inClusterConfig := &rest.Config{
		Host:        fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		BearerToken: bearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	config := client.Config{
		Logger:          log.NewStdErr(true),
		EndpoinURL:      s.URL,
		InClusterConfig: inClusterConfig,
		K8sClient:       c,
	}
	return config
}
