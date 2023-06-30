package client_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
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

func Test_Client_Read(t *testing.T) {
	t.Parallel()

	timeout := 200 * time.Millisecond
	server := testKsmEndpoint(t)

	cpClient, err := client.New(client.WithMaxRetries(0), client.WithTimeout(timeout))
	require.NoError(t, err)

	familyGetter := cpClient.MetricFamiliesGetFunc(server.URL)
	query := prometheus.Query{
		MetricName: "kube_pod_status_phase",
	}
	queries := []prometheus.Query{query}

	families, err := familyGetter(queries)
	require.NoError(t, err)

	// stateset parser failure, did not prevent kube_pod_status_phase from being reported
	require.Equal(t, len(families), 1)
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

/**
 * This endpoint simulates a potential response from the KSM
 * metrics endpoint. There are two metric families present. One
 * with TYPE gauge and another with TYPE stateset. Stateset is
 * not a currently supported type in our parsing so parsing
 * should immediately fail when the type is seen. Because the
 * gauge metric family occurs before the stateset metric family
 * we expect to successfully parse the gauge metrics before
 * parsing failure occurs.
 */
func testKsmEndpoint(t *testing.T) *httptest.Server {
	t.Helper()

	testServer := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, err := io.WriteString(w,
				`# HELP kube_pod_status_phase The pods current phase. 
				 # TYPE kube_pod_status_phase gauge
				 kube_pod_status_phase{namespace="default",pod="123456789"} 1
				 # HELP kube_custom_elasticsearch_health_status Elasticsearch CRD health status
				 # TYPE kube_custom_elasticsearch_health_status stateset
				 kube_custom_elasticsearch_health_status {customresource_group="elasticsearch.k8s.elastic.co"} 1
				`)
			assert.Nil(t, err)
		}))

	t.Cleanup(func() {
		testServer.Close()
	})

	return testServer
}
