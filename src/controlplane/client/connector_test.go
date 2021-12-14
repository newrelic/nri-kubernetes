package client_test

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const (
	testValidURL   = "https://test:443"
	testInvalidURL = ":invalid/url:"
)

func Test_DefaultConnector_fails_when_endpoint(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name      string
		endpoints []config.Endpoint
		assert    func(*testing.T, error)
	}{
		{
			name:      "no_endpoints_added",
			endpoints: []config.Endpoint{},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "endpoints cannot be empty")
			},
		},
		{
			name: "has_invalid_url",
			endpoints: []config.Endpoint{{
				URL: testInvalidURL,
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid url should fail")
			},
		},
		{
			name: "has_invalid_auth",
			endpoints: []config.Endpoint{{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "invalid",
				},
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid auth should fail")
			},
		},
		{
			name: "mTLS_has_not_config",
			endpoints: []config.Endpoint{{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "mTLS",
				},
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "if type mTLS is set mTLS auth must be set")
			},
		},
		{
			name: "mTLS_has_no_secret",
			endpoints: []config.Endpoint{{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "mTLS",
					MTLS: &config.MTLS{
						TLSSecretName: "",
					},
				},
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "secret cannot be empty")
			},
		},
	}

	for _, test := range tt {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := client.DefaultConnector(
				test.endpoints,
				fake.NewSimpleClientset(),
				&rest.Config{},
				log.Discard,
			)
			test.assert(t, err)
		})
	}
}
