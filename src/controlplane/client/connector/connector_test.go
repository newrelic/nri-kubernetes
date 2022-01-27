package connector_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/authenticator"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/client/connector"
)

const (
	prometheusPath = "/metrics"
)

func Test_Connector_probes_endpoints_list(t *testing.T) {
	t.Parallel()

	hitsOKServer := 0
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsOKServer++
		if r.URL.Path == prometheusPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer okServer.Close()

	hitsFailServer := 0
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		hitsFailServer++
	}))
	defer failServer.Close()

	hitsSkippedServer := 0
	skippedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		hitsSkippedServer++
	}))
	defer skippedServer.Close()

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

	authenticator, err := authenticator.New(authenticator.Config{
		InClusterConfig: &rest.Config{},
	})
	assert.NoError(t, err)

	connector, err := connector.New(
		connector.Config{
			Authenticator: authenticator,
			Endpoints:     endpoints,
		},
	)
	assert.NoError(t, err)

	_, err = connector.Connect()
	assert.NoError(t, err)

	assert.Equal(t, 1, hitsOKServer)
	assert.Equal(t, 2, hitsFailServer)
	// Endpoints are not probed after first successful probe.
	assert.Equal(t, 0, hitsSkippedServer)
}

func Test_Connect_fails_when(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		endpoints []config.Endpoint
		assert    func(*testing.T, error)
	}{
		{
			name:      "all_probes_fail",
			endpoints: []config.Endpoint{{URL: "https://fail:1234"}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "connect must fail if all probes fails")
			},
		},
		{
			name:      "no_endpoint_in_the_list",
			endpoints: []config.Endpoint{},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "empty endpoint list must fail")
			},
		},
		{
			name: "has_empty_url",
			endpoints: []config.Endpoint{{
				URL: "",
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "empty url must fail")
			},
		},
		{
			name: "has_invalid_url_format",
			endpoints: []config.Endpoint{{
				URL: ":invalid/url:",
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid url must fail")
			},
		},
		{
			name: "fails_to_authenticate",
			endpoints: []config.Endpoint{{
				URL: "https://mTLSendpoint:443",
				// missing MTLS auth config
				Auth: &config.Auth{Type: "mTLS"},
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid url must fail")
			},
		},
	}

	for _, tc := range testCases {
		test := tc

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			authenticator, err := authenticator.New(authenticator.Config{})
			assert.NoError(t, err)

			connector, err := connector.New(
				connector.Config{
					Authenticator: authenticator,
					Endpoints:     test.endpoints,
				},
			)
			assert.NoError(t, err)

			_, err = connector.Connect()
			test.assert(t, err)
		})
	}
}
