package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"sync"
	"testing"

	"github.com/newrelic/nri-kubernetes/v2/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
)

const (
	healthz          = "/healthz"
	apiProxy         = "/api/v1/nodes/test-node/proxy"
	prometheusMetric = "/metric"
	kubeletMetric    = "/kubelet-metric"
	nodeName         = "test-node"
)

func TestClientCalls(t *testing.T) {
	s, requests := testHTTPSServerWithEndpoints(t, []string{healthz, prometheusMetric, kubeletMetric})

	k8sClient, cf, inClusterConfig := getTestData(s)

	kubeletClient, err := client.New(client.DefaultConnector(k8sClient, cf, inClusterConfig, logutil.Debug), client.WithLogger(logutil.Debug))

	t.Run("creation_succeeds_receiving_200", func(t *testing.T) {
		require.NoError(t, err)
	})

	t.Run("hits_only_local_kubelet", func(t *testing.T) {
		require.NotNil(t, requests)

		_, found := requests[healthz]
		assert.True(t, found)
	})

	t.Run("hits_kubelet_metric", func(t *testing.T) {
		r, err := kubeletClient.Get(kubeletMetric)
		assert.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)

		_, found := requests[kubeletMetric]
		assert.True(t, found)
	})

	t.Run("hits_prometheus_metric", func(t *testing.T) {

		f := kubeletClient.MetricFamiliesGetFunc(prometheusMetric)
		_, err = f(nil)

		r, found := requests[prometheusMetric]
		assert.True(t, found)
		assert.Equal(t, "text/plain", r.Header["Accept"][0])
	})
}

func TestClientCallsViaAPIProxy(t *testing.T) {
	t.Parallel()

	s, requests := testHTTPSServerWithEndpoints(t,
		[]string{path.Join(apiProxy, healthz), path.Join(apiProxy, prometheusMetric), path.Join(apiProxy, kubeletMetric)},
	)

	k8sClient, cf, inClusterConfig := getTestData(s)
	cf.NodeIP = "invalid" // disabling local connection

	kubeletClient, err := client.New(client.DefaultConnector(k8sClient, cf, inClusterConfig, logutil.Debug), client.WithLogger(logutil.Debug))

	t.Run("creation_succeeds_receiving_200", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, err)
	})

	t.Run("hits_api_server_as_fallback", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, requests)

		_, found := requests[path.Join(apiProxy, healthz)]
		assert.True(t, found)
	})

	t.Run("hits_kubelet_metric_through_proxy", func(t *testing.T) {
		t.Parallel()

		r, err := kubeletClient.Get(kubeletMetric)
		assert.NoError(t, err)
		assert.Equal(t, r.StatusCode, http.StatusOK)

		_, found := requests[path.Join(apiProxy, kubeletMetric)]
		assert.True(t, found)
	})

	t.Run("hits_prometheus_metric_through_proxy", func(t *testing.T) {
		t.Parallel()

		f := kubeletClient.MetricFamiliesGetFunc(prometheusMetric)
		_, err = f(nil)

		r, found := requests[path.Join(apiProxy, prometheusMetric)]
		assert.True(t, found)

		assert.Equal(t, "text/plain", r.Header["Accept"][0])
	})

	t.Run("do_not_hit_prometheus_endpoint", func(t *testing.T) {
		t.Parallel()

		f := kubeletClient.MetricFamiliesGetFunc("not-existing")
		_, err = f(nil)
		assert.Error(t, err)
	})
}

func TestConfigPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("connector_takes_scheme_from_config", func(t *testing.T) {
		t.Parallel()

		s, _ := testHTTPSServerWithEndpoints(t, []string{healthz, prometheusMetric, kubeletMetric})
		k8sClient, cf, inClusterConfig := getTestData(s)
		cf.Kubelet.Scheme = "http"

		_, err := client.New(client.DefaultConnector(k8sClient, cf, inClusterConfig, logutil.Debug), client.WithLogger(logutil.Debug))
		require.Error(t, err)
	})

	t.Run("connector_takes_port_from_config", func(t *testing.T) {
		t.Parallel()

		s, _ := testHTTPServerWithEndpoints(t, []string{healthz, prometheusMetric, kubeletMetric})
		_, cf, inClusterConfig := getTestData(s)

		// We use an empty client, but the connector is retrieving the port from the config.
		k8sClient := fake.NewSimpleClientset()
		u, _ := url.Parse(s.URL)
		port, _ := strconv.Atoi(u.Port())
		cf.Kubelet.Port = int32(port)

		_, err := client.New(client.DefaultConnector(k8sClient, cf, inClusterConfig, logutil.Debug), client.WithLogger(logutil.Debug))
		require.NoError(t, err)
	})
}

func TestClientFailingProbingHTTP(t *testing.T) {
	t.Parallel()

	s, requests := testHTTPServerWithEndpoints(t, []string{})

	c, cf, inClusterConfig := getTestData(s)

	_, err := client.New(client.DefaultConnector(c, cf, inClusterConfig, logutil.Debug))

	t.Run("fails_receiving_404", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, err)
	})

	t.Run("hits_both_api_server_and_local_kubelet", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, requests)

		_, found := requests[path.Join(apiProxy, healthz)]
		assert.True(t, found)

		_, found = requests[healthz]
		assert.True(t, found)
	})

	t.Run("does_not_attach_bearer_token", func(t *testing.T) {
		t.Parallel()

		var expectedEmptySlice []string
		assert.Equal(t, expectedEmptySlice, requests[healthz].Header["Authorization"])
	})
}

func TestClientFailingProbingHTTPS(t *testing.T) {
	t.Parallel()

	s, requests := testHTTPSServerWithEndpoints(t, []string{})

	c, cf, inClusterConfig := getTestData(s)

	_, err := client.New(client.DefaultConnector(c, cf, inClusterConfig, logutil.Debug))

	t.Run("fails_receiving_404", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, err)
	})

	t.Run("hits_both_api_server_and_local_kubelet", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, requests)

		_, found := requests[path.Join(apiProxy, healthz)]
		assert.True(t, found)

		_, found = requests[healthz]
		assert.True(t, found)
	})

	t.Run("does_not_attach_bearer_token", func(t *testing.T) {
		t.Parallel()

		for _, v := range requests {
			assert.Equal(t, "Bearer 12345", v.Header["Authorization"][0])
		}
	})
}

func TestClientOptions(t *testing.T) {
	t.Parallel()

	s, _ := testHTTPSServerWithEndpoints(t, []string{healthz, prometheusMetric, kubeletMetric})

	k8sClient, cf, inClusterConfig := getTestData(s)

	_, err := client.New(client.DefaultConnector(k8sClient, cf, inClusterConfig, logutil.Debug), client.WithLogger(logutil.Debug))

	assert.NoError(t, err)
}

func getTestData(s *httptest.Server) (*fake.Clientset, *config.Config, *rest.Config) {
	u, _ := url.Parse(s.URL)
	port, _ := strconv.Atoi(u.Port())

	c := fake.NewSimpleClientset(getTestNode(port))

	cf := &config.Config{
		NodeName: nodeName,
		NodeIP:   u.Hostname(),
	}

	inClusterConfig := &rest.Config{
		Host:        fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		BearerToken: "12345",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	return c, cf, inClusterConfig
}

func getTestNode(port int) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			DaemonEndpoints: v1.NodeDaemonEndpoints{
				KubeletEndpoint: v1.DaemonEndpoint{
					Port: int32(port),
				},
			},
		},
	}
}

func testHTTPServerWithEndpoints(t *testing.T, endpoints []string) (*httptest.Server, map[string]*http.Request) {
	t.Helper()

	requestsReceived := map[string]*http.Request{}
	l := sync.Mutex{}

	testServer := httptest.NewServer(handler(&l, requestsReceived, endpoints))

	return testServer, requestsReceived
}

func testHTTPSServerWithEndpoints(t *testing.T, endpoints []string) (*httptest.Server, map[string]*http.Request) {
	t.Helper()

	requestsReceived := map[string]*http.Request{}
	l := sync.Mutex{}

	testServer := httptest.NewTLSServer(handler(&l, requestsReceived, endpoints))

	return testServer, requestsReceived
}

func handler(l sync.Locker, requestsReceived map[string]*http.Request, endpoints []string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		l.Lock()
		requestsReceived[r.RequestURI] = r
		l.Unlock()

		for _, e := range endpoints {
			if e == r.RequestURI {
				rw.WriteHeader(200)
				return
			}
		}
		rw.WriteHeader(404)
	}
}
