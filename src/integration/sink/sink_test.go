package sink_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v3/src/integration/sink"
)

const (
	defaultRequestTimeout = 1 * time.Second
	retries               = 3
)

func Test_http_Sink_creation_fails_when_there_is(t *testing.T) {
	t.Parallel()

	testCases := map[string]func(s *sink.HTTPSinkOptions){
		"no_client": func(s *sink.HTTPSinkOptions) {
			s.Client = nil
		},
		"no_url": func(s *sink.HTTPSinkOptions) {
			s.URL = ""
		},
	}

	for testName, modifyFunc := range testCases {
		modifyFunc := modifyFunc

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			options := getHTTPSinkOptions(t)
			modifyFunc(&options)

			_, err := sink.New(options)
			assert.Error(t, err, "error expected since client is nil")
		})
	}
}

func Test_http_sink_writes_data_successfully_when_within_ctxDeadline(t *testing.T) {
	t.Parallel()

	numRetries := 1

	testCases := map[string]func(w http.ResponseWriter, req *http.Request){
		"server_returns_204": func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(204)
			_, _ = w.Write([]byte("randomData"))
		},
		"server_returns_5xx_and_then_204": func(w http.ResponseWriter, req *http.Request) {
			if numRetries < retries {
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

			options := getHTTPSinkOptions(t)
			options.URL = testURL

			h, err := sink.New(options)
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
			requestTimeout: defaultRequestTimeout,
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

			c := defaultPesterClient(t)
			c.Timeout = tc.requestTimeout

			h, err := sink.New(sink.HTTPSinkOptions{
				URL:    testURL,
				Client: c,
			})
			require.NoError(t, err, "no error expected")

			_, err = h.Write([]byte("random data"))
			assert.Error(t, err, "error expected")
		})
	}
}

func runTestServer(t *testing.T, testHandler func(w http.ResponseWriter, req *http.Request)) string {
	t.Helper()

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
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

func getHTTPSinkOptions(t *testing.T) sink.HTTPSinkOptions {
	t.Helper()

	return sink.HTTPSinkOptions{
		URL:    sink.DefaultAgentForwarderhost,
		Client: defaultPesterClient(t),
	}
}

func defaultPesterClient(t *testing.T) *pester.Client {
	t.Helper()

	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = retries
	c.Timeout = defaultRequestTimeout
	c.LogHook = func(e pester.ErrEntry) {
		log.Warn(e)
	}

	return c
}
