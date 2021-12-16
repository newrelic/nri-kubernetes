package client_test

import (
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
	testValidURL = "https://test:443"
)

func Test_Connect_fails_when(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name      string
		endpoints []config.Endpoint
		assert    func(*testing.T, error)
	}{
		{
			name:      "no_endpoint_in_the_list",
			endpoints: []config.Endpoint{},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "empty endpoint list should fail")
			},
		},
		{
			name: "has_empty_url",
			endpoints: []config.Endpoint{{
				URL: "",
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "empty url should fail")
			},
		},
		{
			name: "has_invalid_url_format",
			endpoints: []config.Endpoint{{
				URL: ":invalid/url:",
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid url should fail")
			},
		},
		{
			name: "has_unknown_auth_type",
			endpoints: []config.Endpoint{{
				URL: testValidURL,
				Auth: &config.Auth{
					Type: "unknown auth type",
				},
			}},
			assert: func(t *testing.T, err error) {
				require.Error(t, err, "invalid auth should fail")
			},
		},
		{
			name: "mTLS_type_is_selected_but_has_not_mTLS_auth_config",
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
			name: "mTLS_auth_config_has_no_TLSSecretName",
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

			connector, err := client.DefaultConnector(
				test.endpoints,
				fake.NewSimpleClientset(),
				&rest.Config{},
				log.Discard,
			)
			assert.NoError(t, err)

			_, err = connector.Connect()
			test.assert(t, err)
		})
	}
}
