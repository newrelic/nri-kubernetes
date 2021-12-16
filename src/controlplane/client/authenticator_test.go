package client_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

const (
	testValidURL = "https://test:443"
	bearerToken  = "12345"
)

func Test_Authenticate_for_http_endpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	authenticator := client.NewAuthenticator(log.Discard, nil, &rest.Config{})

	rt, err := authenticator.Authenticate(config.Endpoint{URL: server.URL})
	assert.NoError(t, err)

	c := &http.Client{Transport: rt}

	_, err = c.Get(server.URL)
	assert.NoError(t, err)
}

func Test_Authenticate_for_https_endpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	authenticator := client.NewAuthenticator(log.Discard, nil, &rest.Config{})

	endpoint := config.Endpoint{
		URL:                server.URL,
		InsecureSkipVerify: true,
	}

	rt, err := authenticator.Authenticate(endpoint)
	assert.NoError(t, err)

	c := &http.Client{Transport: rt}

	_, err = c.Get(server.URL)
	assert.NoError(t, err)
}

func Test_Authenticate_for_https_endpoint_with_bearer_token_auth(t *testing.T) {
	t.Parallel()

	server := testHTTPSServerBearer(t)

	authenticator := client.NewAuthenticator(log.Discard, nil, &rest.Config{BearerToken: bearerToken})

	endpoint := config.Endpoint{
		URL:                server.URL,
		InsecureSkipVerify: true,
		Auth: &config.Auth{
			Type: "bearer",
		},
	}

	rt, err := authenticator.Authenticate(endpoint)
	assert.NoError(t, err)

	c := &http.Client{Transport: rt}

	resp, err := c.Get(server.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_Authenticator_fails_when(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		endpoint config.Endpoint
		assert   func(*testing.T, error)
	}{
		{
			name: "has_unknown_auth_type",
			endpoint: config.Endpoint{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "unknown auth type",
				},
			},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid auth should fail")
			},
		},
		{
			name: "mTLS_type_is_selected_but_has_not_mTLS_auth_config",
			endpoint: config.Endpoint{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "mTLS",
				},
			},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "if type mTLS is set mTLS auth must be set")
			},
		},
		{
			name: "mTLS_auth_config_has_no_TLSSecretName",
			endpoint: config.Endpoint{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "mTLS",
					MTLS: &config.MTLS{
						TLSSecretName: "",
					},
				},
			},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "secret cannot be empty")
			},
		},
	}

	for _, tc := range testCases {
		test := tc

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			authenticator := client.NewAuthenticator(log.Discard, nil, &rest.Config{})

			_, err := authenticator.Authenticate(test.endpoint)
			test.assert(t, err)
		})
	}
}

func testHTTPSServerBearer(t *testing.T) *httptest.Server {
	t.Helper()

	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), bearerToken) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}
