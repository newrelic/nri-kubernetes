package metric

import (
	"fmt"
	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

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

func TestFetchFuncasdf(test *testing.T) {
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

func TestBuildsHostFromEnvVars(test *testing.T) {
	expectedIP := "123:45:67:89"
	expectedPort := "1011"
	expectedHost := fmt.Sprintf("https://%s:%s", expectedIP, expectedPort)

	os.Setenv("KUBERNETES_SERVICE_HOST", expectedIP)
	os.Setenv("KUBERNETES_SERVICE_PORT", expectedPort)

	assert.Equal(test, expectedHost, getKubeServiceHost())
}

func TestShouldUseKubeServiceURL(t *testing.T) {
	expectedIP := "111:222:33:44"
	expectedPort := "5555"
	nodeName := "my_Node"
	expectedURL := fmt.Sprintf("https://%s:%s/api/v1/pods?fieldSelector=spec.nodeName=%s", expectedIP, expectedPort, nodeName)
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

	assert.NoError(t, err)
	assert.Equal(t, expectedURL, scrapedURL)
}

func TestShouldUseKubeletURL(t *testing.T) {
	scrapedURL := ""
	c := testClient{
		handler: func(writer http.ResponseWriter, request *http.Request) {
			scrapedURL = request.URL.String()
			servePayload(writer, request)
		},
	}

	podFetch := NewBasicPodsFetcher(logutil.Debug, &c)
	_, err := podFetch.DoPodsFetch()

	assert.NoError(t, err)
	assert.Equal(t, "https://127.0.0.1:738/pods", scrapedURL)
}

func TestShouldAlsoUseKubeletURL(t *testing.T) {
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

	assert.NoError(t, err)
	assert.Equal(t, "https://127.0.0.1:738/pods", scrapedURL)
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
