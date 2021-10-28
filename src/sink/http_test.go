package sink_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sethgrid/pester"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-kubernetes/v2/src/sink"
)

func Test_HTTPSink_Creation_Error_NoClient(t *testing.T) {
	t.Parallel()

	_, err := sink.NewHTTPSink(context.Background(), nil, sink.DefaultAgentForwarderEndpoint, sink.DefaultTimeout)
	require.Error(t, err, "error expected since client is nil")
}

func Test_HTTPSink_Creation_Error_No_URL(t *testing.T) {
	t.Parallel()

	_, err := sink.NewHTTPSink(context.Background(), pester.New(), "", sink.DefaultTimeout)
	require.Error(t, err, "error expected url client is empty")
}

func Test_HTTPSink_Creation_Error_No_CtxTimeout(t *testing.T) {
	t.Parallel()

	_, err := sink.NewHTTPSink(context.Background(), pester.New(), sink.DefaultAgentForwarderEndpoint, 0)
	require.Error(t, err, "error expected since timeout is zero")
}

func Test_HTTP_Sink_Creation_Error_No_Ctx(t *testing.T) {
	t.Parallel()

	_, err := sink.NewHTTPSink(nil, pester.New(), sink.DefaultAgentForwarderEndpoint, sink.DefaultRequestTimeout)
	require.Error(t, err, "error expected since ctx is null")
}

func Test_Default_Pester_Client(t *testing.T) {
	t.Parallel()

	c := sink.DefaultPesterClient(sink.DefaultRequestTimeout)

	require.NotNil(t, c)
	assert.Equal(t, 5, c.MaxRetries)
	assert.Equal(t, sink.DefaultRequestTimeout, c.Timeout)
}

func Test_HTTPSink_writes_data_successfully_when_server_return_204(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		testHandler func(w http.ResponseWriter, req *http.Request)
	}{
		"server_return_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(204)
				_, _ = w.Write([]byte("randomData"))
			},
		},
	}

	for testName, testcase := range testCases {
		tc := testcase

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			testHandler := tc.testHandler
			testURL := runTestServer(t, testHandler)

			c := sink.DefaultPesterClient(sink.DefaultRequestTimeout)
			h, err := sink.NewHTTPSink(context.Background(), c, testURL, sink.DefaultTimeout)
			require.NoError(t, err, "no error expected")

			_, err = h.Write([]byte("random data"))
			assert.NoError(t, err, "no error expected")
		})
	}
}

func Test_Default_Pester_Client_Writing_Data_Is_Retried(t *testing.T) {
	t.Parallel()

	numRetries := 0

	testHandler := func(w http.ResponseWriter, req *http.Request) {
		if numRetries < 2 {
			numRetries++
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}

	testURL := runTestServer(t, testHandler)

	c := sink.DefaultPesterClient(sink.DefaultRequestTimeout)
	h, err := sink.NewHTTPSink(context.Background(), c, testURL, sink.DefaultTimeout)
	require.NoError(t, err, "no error expected")

	_, err = h.Write([]byte("random data"))
	assert.NoError(t, err, "no error expected")

}

func Test_HTTPSink_fails_writing_data_when(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		testHandler    func(w http.ResponseWriter, req *http.Request)
		requestTimeout time.Duration
	}{
		"data_post_fails_when_server_never_returns_204": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(404)
			},
			requestTimeout: sink.DefaultRequestTimeout,
		},
		"data_post_fails_when_reply_exceeds_context_deadline": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(3 * time.Second)
				w.WriteHeader(204)
			},
			requestTimeout: sink.DefaultRequestTimeout,
		},
		"data_post_fails_when_replies_exceed_request_timeout": {
			testHandler: func(w http.ResponseWriter, req *http.Request) {
				time.Sleep(3 * time.Second)
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

			c := sink.DefaultPesterClient(tc.requestTimeout)
			h, err := sink.NewHTTPSink(context.Background(), c, testURL, 1*time.Second)
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
