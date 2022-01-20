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
	hit := false
	server := testHTTPServer(t, &timeout, &hit)

	authenticator, err := authenticator.New(authenticator.Config{})
	assert.NoError(t, err)

	c, err := connector.New(
		connector.Config{
			Authenticator: authenticator,
			Endpoints:     []config.Endpoint{{URL: server.URL}},
			Timeout:       2000 * time.Millisecond,
		},
	)
	assert.NoError(t, err)

	cpClient, err := client.New(c, client.WithMaxRetries(0))
	require.NoError(t, err)

	familyGetter := cpClient.MetricFamiliesGetFunc()

	// Scrapes prometheus endpoint
	_, err = familyGetter(nil)
	require.NoError(t, err)
	require.Equal(t, true, hit)

	// Overwrite httpServer timeout to be higher than client's
	timeout = timeout + c.Timeout

	// Fails if timeout
	_, err = familyGetter(nil)
	require.Error(t, err)

	// Test calling retry
	cpClientWithReries, err := client.New(c, client.WithMaxRetries(2))
	require.NoError(t, err)

	familyGetter = cpClientWithReries.MetricFamiliesGetFunc()

	// Overwrite httpServer timeout to be higher than client's
	timeout = timeout + c.Timeout

	// Should not fail because of second retry
	_, err = familyGetter(nil)
	require.NoError(t, err)
}

func testHTTPServer(t *testing.T, sleepDuration *time.Duration, hit *bool) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sleepDuration != nil {
			time.Sleep(*sleepDuration)
			// resetting the duration to 1 millisecond for second client retry to succeed if timeout is higher
			*sleepDuration = time.Millisecond
		}

		if r.URL.Path == prometheusPath {
			*hit = true
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
