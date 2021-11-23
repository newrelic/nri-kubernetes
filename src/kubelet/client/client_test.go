package client_test

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"sync"
	"testing"

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

	kubeletClient, err := client.New(k8sClient, cf, inClusterConfig)

	t.Run("creation_succeeds_receiving_200", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("hits_only_local_kubelet", func(t *testing.T) {
		require.NotNil(t, requests)
		require.Len(t, requests, 1)

		_, found := requests[healthz]
		assert.True(t, found)
	})

	t.Run("hits_kubelet_metric", func(t *testing.T) {
		_, err := kubeletClient.Get(kubeletMetric)

		assert.NoError(t, err)

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

func TestClientCallsWithAPIProxy(t *testing.T) {
	s, requests := testHTTPSServerWithEndpoints(t, []string{path.Join(apiProxy, healthz), prometheusMetric, kubeletMetric})

	k8sClient, cf, inClusterConfig := getTestData(s)
	cf.NodeIP = "invalid" // disabling local connection

	kubeletClient, err := client.New(k8sClient, cf, inClusterConfig)

	t.Run("creation_succeeds_receiving_200", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("hits_api_server_as_fallback", func(t *testing.T) {
		require.NotNil(t, requests)
		require.Len(t, requests, 1)

		_, found := requests[path.Join(apiProxy, healthz)]
		assert.True(t, found)
	})

	t.Run("hits_kubelet_metric_through_proxy", func(t *testing.T) {
		_, err := kubeletClient.Get(kubeletMetric)

		assert.NoError(t, err)

		_, found := requests[path.Join(apiProxy, kubeletMetric)]
		assert.True(t, found)
	})

	t.Run("hits_prometheus_metric_through_proxy", func(t *testing.T) {

		f := kubeletClient.MetricFamiliesGetFunc(prometheusMetric)
		_, err = f(nil)

		r, found := requests[path.Join(apiProxy, prometheusMetric)]
		assert.True(t, found)

		assert.Equal(t, "text/plain", r.Header["Accept"][0])
	})
}

func TestClientFailingProbingHTTP(t *testing.T) {
	s, requests := testHTTPServerWithEndpoints(t, []string{})

	c, cf, inClusterConfig := getTestData(s)

	_, err := client.New(c, cf, inClusterConfig)

	t.Run("fails_receiving_404", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, err)
	})

	t.Run("hits_both_api_server_and_local_kubelet", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, requests)
		require.Len(t, requests, 2)

		_, found := requests[path.Join(apiProxy, healthz)]
		assert.True(t, found)

		_, found = requests[healthz]
		assert.True(t, found)
	})

	t.Run("does_not_attach_bearer_token", func(t *testing.T) {
		t.Parallel()

		var expectedEmptySlice []string
		for _, v := range requests {
			assert.Equal(t, expectedEmptySlice, v.Header["Authorization"])
		}
	})
}

func TestClientFailingProbingHTTPS(t *testing.T) {
	s, requests := testHTTPSServerWithEndpoints(t, []string{})

	c, cf, inClusterConfig := getTestData(s)

	_, err := client.New(c, cf, inClusterConfig)

	t.Run("fails_receiving_404", func(t *testing.T) {
		t.Parallel()

		assert.Error(t, err)
	})

	t.Run("hits_both_api_server_and_local_kubelet", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, requests)
		require.Len(t, requests, 2)

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
	s, _ := testHTTPSServerWithEndpoints(t, []string{healthz, prometheusMetric, kubeletMetric})

	k8sClient, cf, inClusterConfig := getTestData(s)

	_, err := client.New(k8sClient, cf, inClusterConfig, client.WithLogger(log.NewStdErr(true)))

	assert.NoError(t, err)
}

func getTestData(s *httptest.Server) (*fake.Clientset, config.Mock, *rest.Config) {
	u, _ := url.Parse(s.URL)
	port, _ := strconv.Atoi(u.Port())

	c := fake.NewSimpleClientset(getTestNode(port))

	cf := config.Mock{
		NodeName: nodeName,
		NodeIP:   u.Hostname(),
	}

	inClusterConfig := &rest.Config{
		Host:        fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		BearerToken: "12345",
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

func handler(l *sync.Mutex, requestsReceived map[string]*http.Request, endpoints []string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		l.Lock()
		requestsReceived[r.RequestURI] = r
		l.Unlock()

		for _, e := range endpoints {
			if e == r.RequestURI {
				rw.WriteHeader(200)
			}
		}
		rw.WriteHeader(404)
	}
}
