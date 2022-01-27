package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/ksm/client"
)

func Test_Client(t *testing.T) {
	t.Parallel()

	var requestsReceived int

	timeout := 200 * time.Millisecond
	server := testHTTPServer(t, &requestsReceived, timeout*2)

	cpClient, err := client.New(client.WithMaxRetries(0), client.WithTimeout(timeout))
	require.NoError(t, err)

	familyGetter := cpClient.MetricFamiliesGetFunc(server.URL)

	// Fails if timeout
	_, err = familyGetter(nil)
	require.Error(t, err)

	// Test calling retry
	cpClient, err = client.New(client.WithMaxRetries(4), client.WithTimeout(timeout))
	require.NoError(t, err)

	familyGetter = cpClient.MetricFamiliesGetFunc(server.URL)

	// Should retry and not fail with timeout
	_, err = familyGetter(nil)
	require.NoError(t, err)

	require.Equal(t, 3, requestsReceived)
}

func testHTTPServer(t *testing.T, requestsReceived *int, timeout time.Duration) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requestsReceived++
		if *requestsReceived < 3 {
			time.Sleep(timeout)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}
