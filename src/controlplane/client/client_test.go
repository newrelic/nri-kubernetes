package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

	server := testHTTPServer(t, nil)

	endpoints := []config.Endpoint{{URL: server.URL}}

	cpClient, err := client.New(
		getTestConnector(server, endpoints),
		client.WithLogger(log.Discard),
	)
	assert.NoError(t, err)

	t.Run("return_ok_when_succeed", func(t *testing.T) {
		r, err := cpClient.Get(prometheusPath)
		require.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)
	})
}

func Test_Client_tries_endpoints_list(t *testing.T) {
	t.Parallel()

	okServer := testHTTPServer(t, nil)
	requestCountFail := 0
	failServer := testHTTPServerFail(t, &requestCountFail)
	requestCountSkipped := 0
	skippedServer := testHTTPServerFail(t, &requestCountSkipped)

	endpoints := []config.Endpoint{
		// Failing endpoints.
		{
			URL: failServer.URL,
		},
		{
			URL:  failServer.URL,
			Auth: &config.Auth{Type: "bearer"},
		},
		{
			URL: "http://localhost:1234",
		},
		// Working endpoint.
		{
			URL: okServer.URL,
		},
		// This endpoint must not be hit.
		{
			URL: skippedServer.URL,
		},
	}

	cpClient, err := client.New(getTestConnector(okServer, endpoints))
	assert.NoError(t, err)

	r, err := cpClient.Get(prometheusPath)
	require.NoError(t, err)
	assert.Equal(t, r.StatusCode, http.StatusOK)
	//
	assert.Equal(t, 2, requestCountFail)
	// Endpoints are not probed after first successful probe.
	assert.Equal(t, 0, requestCountSkipped)
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

func Test_Client_fails_scraping_timeout(t *testing.T) {
	t.Parallel()

	timeout := time.Millisecond

	server := testHTTPServer(t, &timeout)

	cpClient, err := client.New(getTestConnector(server, []config.Endpoint{{URL: server.URL}}))
	require.NoError(t, err)

	timeout = timeout + client.DefaultTimout

	_, err = cpClient.Get("")
	require.Error(t, err)
}

func Test_New_client_fails(t *testing.T) {
	t.Parallel()
	t.Run("when_unique_endpoint_fails", func(t *testing.T) {
		t.Parallel()

		timeout := client.DefaultTimout + time.Millisecond
		server := testHTTPServer(t, &timeout)

		_, err := client.New(getTestConnector(server, []config.Endpoint{{URL: server.URL}}))
		require.Error(t, err)
	})

	t.Run("when_initiated_with", func(t *testing.T) {
		t.Parallel()

		tt := []struct {
			name      string
			connector client.Connector
			opts      []client.OptionFunc
			assert    func(*testing.T, error)
		}{
			{
				name: "nil_connector",
				assert: func(t *testing.T, err error) {
					require.Error(t, err, "connector is required")
				},
			},
			{
				name: "nil_logger",
				opts: []client.OptionFunc{client.WithLogger(nil)},
				assert: func(t *testing.T, err error) {
					require.Error(t, err, "logger cannot be nil")
				},
			},
		}

		for _, test := range tt {
			test := test

			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				_, err := client.New(test.connector, test.opts...)
				test.assert(t, err)
			})
		}
	})
}

func testHTTPServer(t *testing.T, sleepDuration *time.Duration) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sleepDuration != nil {
			time.Sleep(*sleepDuration)
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
