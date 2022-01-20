package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
	"github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	t.Parallel()

	timeout := 1 * time.Second
	server := testHTTPServer(t, &timeout)

	cpClient, err := client.New(client.WithMaxRetries(0), client.WithTimeout(200*time.Millisecond))
	require.NoError(t, err)

	familyGetter := cpClient.MetricFamiliesGetFunc(server.URL)

	// Fails if timeout
	_, err = familyGetter(nil)
	require.Error(t, err)

	// Test calling retry
	cpClient, err = client.New(client.WithMaxRetries(4), client.WithTimeout(200*time.Millisecond))
	require.NoError(t, err)

	familyGetter = cpClient.MetricFamiliesGetFunc(server.URL)

	// Overwrite httpServer timeout to be higher than client's
	timeout = 2 * time.Second

	// Should retry and not fail with timeout
	_, err = familyGetter(nil)
	require.NoError(t, err)
}

func testHTTPServer(t *testing.T, sleepDuration *time.Duration) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sleepDuration != nil {
			time.Sleep(*sleepDuration)
			// resetting the duration to 1 millisecond for second client retry to succeed if timeout is higher
			*sleepDuration = time.Millisecond
		}

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}
