package metric

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric/testdata"
)

type testClient struct {
	handler http.HandlerFunc
}

func (c *testClient) Do(method, path string) (*http.Response, error) {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()

	c.handler(w, req)

	return w.Result(), nil
}

func (c *testClient) NodeIP() string {
	// nothing to do
	return ""
}

func servePayload(w http.ResponseWriter, _ *http.Request) {
	f, err := os.Open("testdata/kubelet_pods_payload.json")
	if err != nil {
		panic(err)
	}

	defer f.Close() // nolint: errcheck

	io.Copy(w, f) // nolint: errcheck
}

func serverPanic(w http.ResponseWriter, _ *http.Request) {
	panic(fmt.Errorf("server panic"))
}

func TestFetchFunc(t *testing.T) {
	c := testClient{
		handler: servePayload,
	}

	f := NewPodsFetcher(logrus.StandardLogger(), &c, true)
	g, err := f.FetchFuncWithCache()()

	assert.NoError(t, err)
	assert.Equal(t, testdata.ExpectedRawData, g)
}

func TestFetchFuncCache(t *testing.T) {
	// Given an HTTPClient
	c := testClient{
		handler: servePayload,
	}

	// When calling the fetch pods func the results are cached
	f := NewPodsFetcher(logrus.StandardLogger(), &c, true)
	g, err := f.FetchFuncWithCache()()
	assert.NoError(t, err)
	assert.Equal(t, testdata.ExpectedRawData, g)

	// Subsequent calls will use the cached data
	c = testClient{
		handler: serverPanic,
	}
	f.client = &c
	g, err = f.FetchFuncWithCache()()
	assert.NoError(t, err)
	assert.Equal(t, testdata.ExpectedRawData, g)
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

	f := NewPodsFetcher(logrus.StandardLogger(), &c, true)
	g, err := f.FetchFuncWithCache()()

	assert.EqualError(t, err, errorMessage)
	assert.Empty(t, g)
}
