package sink_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/src/sink"
)

func Test_http_Sink_creation_fails_when_there_is(t *testing.T) {
	t.Parallel()

	testCases := map[string]func(s *sink.HTTPSinkOptions){
		"no_ctx": func(s *sink.HTTPSinkOptions) {
			s.Ctx = nil
		},
		"no_client": func(s *sink.HTTPSinkOptions) {
			s.Client = nil
		},
		"no_url": func(s *sink.HTTPSinkOptions) {
			s.URL = ""
		},
		"no_timeout": func(s *sink.HTTPSinkOptions) {
			s.CtxTimeout = 0
		},
	}

	for testName, modifyFunc := range testCases {
		modifyFunc := modifyFunc

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			options := getHTTPSinkOptions()
			modifyFunc(&options)

			_, err := sink.NewHTTPSink(options)
			assert.Error(t, err, "error expected since client is nil")
		})
	}

}

func Test_default_pester_client_have_expected_parameters(t *testing.T) {
	t.Parallel()

	c := sink.DefaultPesterClient(sink.DefaultRequestTimeout)

	require.NotNil(t, c)
	assert.Equal(t, 5, c.MaxRetries)
	assert.Equal(t, sink.DefaultRequestTimeout, c.Timeout)
}

func Test_http_sink_writes_data_successfully_when_within_ctxDeadline(t *testing.T) {
	t.Parallel()

	numRetries := 0

	testCases := map[string]func(w http.ResponseWriter, req *http.Request){
		"server_returns_204": func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(204)
			_, _ = w.Write([]byte("randomData"))
		},
		"server_returns_5xx_and_then_204": func(w http.ResponseWriter, req *http.Request) {
			if numRetries < 2 {
				numRetries++
				w.WriteHeader(503)
			} else {
				w.WriteHeader(204)
			}
		},
	}

	for testName, testHandler := range testCases {
		testHandler := testHandler

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			testURL := runTestServer(t, testHandler)

			options := getHTTPSinkOptions()
			options.URL = testURL

			h, err := sink.NewHTTPSink(options)
			require.NoError(t, err, "no error expected")

			_, err = h.Write([]byte("random data"))
			assert.NoError(t, err, "no error expected")
		})
	}
}

func Test_http_sink_fails_writing_data_when(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		testHandler    func(w http.ResponseWriter, req *http.Request)
		requestTimeout time.Duration
	}{
		"server_never_returns_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(503)
			},
			requestTimeout: sink.DefaultRequestTimeout,
		},
		"server_replies_after_context_deadline": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(3 * time.Second)
				w.WriteHeader(204)
			},
			requestTimeout: sink.DefaultRequestTimeout,
		},
		"server_replies_to_each_request_after_request_timeout": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(500 * time.Millisecond)
				w.WriteHeader(204)
			},
			requestTimeout: 1 * time.Nanosecond,
		},
	}

	for testName, testcase := range testCases {
		tc := testcase

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			testHandler := tc.testHandler

			testURL := runTestServer(t, testHandler)

			h, err := sink.NewHTTPSink(sink.HTTPSinkOptions{
				URL:        testURL,
				Client:     sink.DefaultPesterClient(tc.requestTimeout),
				CtxTimeout: 1 * time.Second,
				Ctx:        context.Background(),
			})
			require.NoError(t, err, "no error expected")

			_, err = h.Write([]byte("random data"))
			assert.Error(t, err, "error expected")
		})
	}
}

func runTestServer(t *testing.T, testHandler func(w http.ResponseWriter, req *http.Request)) string {
	t.Helper()

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err, "no error expected")

	port := listener.Addr().(*net.TCPAddr).Port
	testURI := fmt.Sprintf("/v1/test/%d", port)

	http.HandleFunc(testURI, testHandler)
	go func() {
		err := http.Serve(listener, nil)
		require.NoError(t, err, "no error expected")
	}()

	return fmt.Sprintf("http://localhost:%d%s", port, testURI)
}

func getHTTPSinkOptions() sink.HTTPSinkOptions {
	return sink.HTTPSinkOptions{
		URL:        sink.DefaultAgentForwarderEndpoint,
		Client:     sink.DefaultPesterClient(sink.DefaultRequestTimeout),
		CtxTimeout: sink.DefaultCtxTimeout,
		Ctx:        context.Background(),
	}
}
