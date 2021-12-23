package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client/authenticator"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane/client/connector"
)

const (
	prometheusPath = "/metrics"
)

func Test_Client(t *testing.T) {
	t.Parallel()

	timeout := time.Millisecond

	server := testHTTPServer(t, &timeout)

	authenticator, err := authenticator.New(authenticator.Config{})
	assert.NoError(t, err)

	c, err := connector.New(
		connector.Config{
			Authenticator: authenticator,
			Endpoints:     []config.Endpoint{{URL: server.URL}},
		},
	)
	assert.NoError(t, err)

	cpClient, err := client.New(c)
	require.NoError(t, err)

	// Scrapes prometheus endpoint
	r, err := cpClient.Get("")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	timeout = timeout + connector.DefaultTimout

	// Fails if timeout
	_, err = cpClient.Get("")
	require.Error(t, err)
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
