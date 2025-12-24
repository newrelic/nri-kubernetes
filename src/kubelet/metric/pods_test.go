package metric

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric/testdata"
)

type testClient struct {
	handler http.HandlerFunc
}

func (c *testClient) Get(urlPath string) (*http.Response, error) {
	uri, _ := url.Parse("https://127.0.0.1:738")
	uri.Path = path.Join(uri.Path, urlPath)

	req := httptest.NewRequest(http.MethodGet, uri.String(), nil)
	return c.Do(req)
}

func (c *testClient) GetURI(url url.URL) (*http.Response, error) {
	req := httptest.NewRequest(http.MethodGet, url.String(), nil)
	return c.Do(req)
}

func (c *testClient) Do(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()

	c.handler(w, req)

	return w.Result(), nil
}

func servePayload(w http.ResponseWriter, _ *http.Request) {
	f, err := os.Open("testdata/kubelet_pods_payload.json")
	if err != nil {
		panic(err)
	}

	defer f.Close() // nolint: errcheck

	io.Copy(w, f) // nolint: errcheck
}

func TestFetchFunc(t *testing.T) {
	c := testClient{
		handler: servePayload,
	}

	f := NewBasicPodsFetcher(logutil.Debug, &c)
	g, err := f.DoPodsFetch()

	assert.NoError(t, err)

	if diff := cmp.Diff(testdata.ExpectedRawData, g); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

func TestFetchFunFromKubeService(test *testing.T) {
	test.Parallel()
	c := testClient{
		handler: servePayload,
	}

	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	os.Setenv("KUBERNETES_SERVICE_PORT", "8080")

	podFetch := NewPodsFetcher(logutil.Debug, &c, &config.Config{
		NodeName: "minicube",
		Kubelet: config.Kubelet{
			FetchPodsFromKubeService: true,
		},
	})
	podFetchResult, err := podFetch.DoPodsFetch()

	assert.NoError(test, err)

	if diff := cmp.Diff(testdata.ExpectedRawData, podFetchResult); diff != "" {
		test.Errorf("unexpected difference: %s", diff)
	}
}

func TestBuildsHostFromEnvVars(test *testing.T) { //nolint: paralleltest
	expectedIP := "123:45:67:89"
	expectedPort := "1011"
	expectedHost := fmt.Sprintf("https://%s:%s", expectedIP, expectedPort) //nolint: nosprintfhostport

	os.Setenv("KUBERNETES_SERVICE_HOST", expectedIP)
	os.Setenv("KUBERNETES_SERVICE_PORT", expectedPort)

	assert.Equal(test, expectedHost, getKubeServiceHost())
}

func TestShouldUseKubeServiceURL(test *testing.T) { //nolint: paralleltest
	expectedIP := "111:222:33:44"
	expectedPort := "5555"
	nodeName := "my_Node"
	expectedURL := fmt.Sprintf("https://%s:%s/api/v1/pods?fieldSelector=spec.nodeName=%s", expectedIP, expectedPort, nodeName) //nolint: nosprintfhostport
	scrapedURL := ""

	os.Setenv("KUBERNETES_SERVICE_HOST", expectedIP)
	os.Setenv("KUBERNETES_SERVICE_PORT", expectedPort)

	c := testClient{
		handler: func(writer http.ResponseWriter, request *http.Request) {
			scrapedURL = request.URL.String()
			servePayload(writer, request)
		},
	}

	podFetch := NewPodsFetcher(logutil.Debug, &c, &config.Config{
		NodeName: nodeName,
		Kubelet: config.Kubelet{
			FetchPodsFromKubeService: true,
		},
	})

	_, err := podFetch.DoPodsFetch()

	assert.NoError(test, err)
	assert.Equal(test, expectedURL, scrapedURL)
}

func TestShouldUseKubeletURLWhenBasicPodsFetcherBuilt(test *testing.T) {
	test.Parallel()
	scrapedURL := ""
	c := testClient{
		handler: func(writer http.ResponseWriter, request *http.Request) {
			scrapedURL = request.URL.String()
			servePayload(writer, request)
		},
	}

	podFetch := NewBasicPodsFetcher(logutil.Debug, &c)
	_, err := podFetch.DoPodsFetch()

	assert.NoError(test, err)
	assert.Equal(test, "https://127.0.0.1:738/pods", scrapedURL)
}

func TestShouldUseKubeletURL(test *testing.T) {
	test.Parallel()
	expectedIP := "111:222:33:44"
	expectedPort := "5555"
	nodeName := "my_Node"
	scrapedURL := ""

	os.Setenv("KUBERNETES_SERVICE_HOST", expectedIP)
	os.Setenv("KUBERNETES_SERVICE_PORT", expectedPort)

	c := testClient{
		handler: func(writer http.ResponseWriter, request *http.Request) {
			scrapedURL = request.URL.String()
			servePayload(writer, request)
		},
	}

	podFetch := NewPodsFetcher(logutil.Debug, &c, &config.Config{
		NodeName: nodeName,
		Kubelet: config.Kubelet{
			FetchPodsFromKubeService: false,
		},
	})

	_, err := podFetch.DoPodsFetch()

	assert.NoError(test, err)
	assert.Equal(test, "https://127.0.0.1:738/pods", scrapedURL)
}

func TestNewPodsFetchFunc_StatusNoOK(t *testing.T) {
	assertError(
		t,
		"error calling kubelet /pods path. Status code 500",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	)
}

func TestNewPodsFetchFunc_ErrorEmptyResponse(t *testing.T) {
	assertError(
		t,
		"error reading response from kubelet /pods path. Response is empty",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("")) // nolint: errcheck
		},
	)
}

func TestNewPodsFetchFunc_ErrorMalformedJSON(t *testing.T) {
	assertError(
		t,
		"error decoding response from kubelet /pods path. invalid character 'P' looking for beginning of value",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("P{}")) // nolint: errcheck
		},
	)
}

func TestOneMetricPerLabel(t *testing.T) {
	g := map[string]string{
		"1": "1",
		"2": "2",
		"3": "3",
	}

	expected := definition.FetchedValues{
		"label.1": "1",
		"label.2": "2",
		"label.3": "3",
	}

	v, err := OneMetricPerLabel(g)
	assert.NoError(t, err)
	assert.Equal(t, expected, v)
}

func assertError(t *testing.T, errorMessage string, handler http.HandlerFunc) {
	c := testClient{
		handler: handler,
	}

	f := NewBasicPodsFetcher(logutil.Debug, &c)
	g, err := f.DoPodsFetch()

	assert.EqualError(t, err, errorMessage)
	assert.Empty(t, g)
}

func TestFetchContainersData_WithSidecarContainers(t *testing.T) {
	t.Parallel()

	startedAt, _ := time.Parse(time.RFC3339, "2025-01-02T15:04:05Z")
	restartPolicyAlways := corev1.ContainerRestartPolicyAlways
	podName := "test-pod"
	namespace := "default"
	sideCarAContainerName := "sidecar"
	sideCarBContainerName := "sidecar-b"
	initContainerName := "normal-init"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:          sideCarAContainerName,
					RestartPolicy: &restartPolicyAlways,
				},
				{
					Name: initContainerName,
				},
				{
					Name:          sideCarBContainerName,
					RestartPolicy: &restartPolicyAlways,
				},
			},
		},
		Status: corev1.PodStatus{
			HostIP: "192.168.0.33",

			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name: sideCarAContainerName,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.NewTime(startedAt),
						},
					},
					Ready:        true,
					RestartCount: 2,
				},
				// Normal init containers should be skipped
				{
					Name: initContainerName,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							FinishedAt: metav1.NewTime(startedAt),
							ExitCode:   0,
							Reason:     "Completed",
						},
					},
					Ready: true,
				},
				{
					Name: sideCarBContainerName,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "ContainerCreating",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							FinishedAt: metav1.NewTime(startedAt),
							ExitCode:   137,
							Reason:     "OOMKilled",
						},
					},
					Ready: false,
				},
			},
		},
	}

	podFetcher := &PodsFetcher{}
	result := podFetcher.fetchContainersData(pod)

	sidecarAID := fmt.Sprintf("%s_%s_%s", namespace, podName, sideCarAContainerName)
	sidecarBID := fmt.Sprintf("%s_%s_%s", namespace, podName, sideCarBContainerName)
	assert.Equal(t, 2, len(result), "expected only the sidecar containers to be processed")
	assert.Contains(t, result, sidecarAID, "expected sidecar to be present as a key")
	assert.Contains(t, result, sidecarBID, "expected sidecar-b to be present as a key")

	assert.Equal(t, "Running", result[sidecarAID]["status"])
	assert.Equal(t, "Waiting", result[sidecarBID]["status"])
	assert.Equal(t, true, result[sidecarAID]["isReady"])
	assert.Equal(t, int32(2), result[sidecarAID]["restartCount"])
	assert.Equal(t, startedAt, result[sidecarAID]["startedAt"])
	assert.Equal(t, "OOMKilled", result[sidecarBID]["lastTerminatedExitReason"])
	assert.Equal(t, int32(137), result[sidecarBID]["lastTerminatedExitCode"])
	assert.Equal(t, "192.168.0.33", result[sidecarAID]["nodeIP"])
	assert.Equal(t, "192.168.0.33", result[sidecarBID]["nodeIP"])
}
