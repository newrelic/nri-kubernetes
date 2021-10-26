package sink_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sethgrid/pester"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v2/src/sink"
)

func TestHTTPSinkCreationError(t *testing.T) {
	t.Parallel()

	_, err := sink.NewHTTPSink(nil, sink.DefaultAgentForwarderEndpoint, sink.DefaultTimeout)
	assert.Error(t, err, "error expected since client is nil")

	_, err = sink.NewHTTPSink(pester.New(), "", sink.DefaultTimeout)
	assert.Error(t, err, "error expected url client is empty")

	_, err = sink.NewHTTPSink(pester.New(), sink.DefaultAgentForwarderEndpoint, 0)
	assert.Error(t, err, "error expected since timeout is zero")
}

func TestHTTPSink_Data_post_succeeds_when(t *testing.T) {
	t.Parallel()

	numRetries := 0

	testCases := map[string]struct {
		testHandler func(w http.ResponseWriter, req *http.Request)
	}{
		"_server_return_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(204)
				_, _ = w.Write([]byte("randomData"))
			},
		},
		"data_post_succeed_when_server_returns_404_and_then_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				if numRetries == 0 {
					numRetries++
					w.WriteHeader(503)
				} else {
					w.WriteHeader(204)
				}
			},
		},
	}

	for testName, testcase := range testCases {
		tc := testcase

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			testHandler := tc.testHandler
			testURL := runTestServer(t, testHandler)

			c := sink.DefaultPesterClient()
			h, err := sink.NewHTTPSink(c, testURL, sink.DefaultTimeout)
			assert.NoError(t, err, "no error expected")

			i, err := integration.New("testIntegration", "0.0.0", integration.Writer(h))
			assert.NoError(t, err, "no error expected")

			err = i.Publish()
			assert.NoError(t, err, "no error expected")
		})
	}
}

func TestHTTPSink_Data_post_fails_when(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		testHandler func(w http.ResponseWriter, req *http.Request)
	}{
		"data_post_fails_when_server_never_returns_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(404)
			},
		},
		"data_post_fails_when_server_takes_too_long_to_answer": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(3 * time.Second)
				w.WriteHeader(204)
			},
		},
	}

	for testName, testcase := range testCases {
		tc := testcase

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			testHandler := tc.testHandler
			testURL := runTestServer(t, testHandler)

			c := sink.DefaultPesterClient()
			h, err := sink.NewHTTPSink(c, testURL, 1*time.Second)
			assert.NoError(t, err, "no error expected")

			i, err := integration.New("testIntegration", "0.0.0", integration.Writer(h))
			assert.NoError(t, err, "no error expected")

			err = i.Publish()
			assert.Error(t, err, "error expected")

		})
	}
}

func runTestServer(t *testing.T, testHandler func(w http.ResponseWriter, req *http.Request)) string {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	assert.NoError(t, err, "no error expected")

	port := listener.Addr().(*net.TCPAddr).Port
	testURI := fmt.Sprintf("/v1/test/%d", port)

	http.HandleFunc(testURI, testHandler)
	go func() {
		err := http.Serve(listener, nil)
		assert.NoError(t, err, "no error expected")
	}()

	return fmt.Sprintf("http://localhost:%d%s", port, testURI)
}
