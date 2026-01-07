// Comprehensive unit tests for connector retry logic
package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
)

// TestConnectorRetry_ImmediateSuccess tests that when connection succeeds immediately,
// no retries are performed and there's minimal overhead.
func TestConnectorRetry_ImmediateSuccess(t *testing.T) {
	t.Parallel()
	var attemptCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	// Configure with retry enabled but connection should succeed immediately
	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 10 * time.Second, // Retry enabled
			InitBackoff: 1 * time.Second,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

	start := time.Now()
	conn, err := connector.Connect()
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, conn)

	// Should succeed very quickly without retries
	assert.Less(t, elapsed, 2*time.Second, "Should connect quickly on immediate success")
	assert.Equal(t, int32(1), atomic.LoadInt32(&attemptCount), "Should make exactly 1 attempt")
}

// TestConnectorRetry_SuccessAfterFailures tests that connection succeeds after several failures.
func TestConnectorRetry_SuccessAfterFailures(t *testing.T) {
	t.Parallel()
	var attemptCount int32
	successOn := int32(3) // Succeed on the 3rd attempt

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < successOn {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 10 * time.Second, // Enough time for 3 attempts
			InitBackoff: 500 * time.Millisecond,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

	start := time.Now()
	conn, err := connector.Connect()
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, conn)

	// Should have made exactly 3 attempts
	assert.Equal(t, successOn, atomic.LoadInt32(&attemptCount), "Should make exactly 3 attempts")

	// Should take at least 2 * backoff (2 failures * 500ms)
	minExpectedTime := 2 * 500 * time.Millisecond
	assert.GreaterOrEqual(t, elapsed, minExpectedTime, "Should have waited for backoff periods")

	// But shouldn't take too long
	assert.Less(t, elapsed, 5*time.Second, "Should not have exceeded timeout")
}

// TestConnectorRetry_TimeoutExceeded tests that connection fails when timeout is exceeded.
func TestConnectorRetry_TimeoutExceeded(t *testing.T) {
	t.Parallel()
	var attemptCount int32

	// Server always fails
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 2 * time.Second, // Short timeout
			InitBackoff: 500 * time.Millisecond,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

	start := time.Now()
	conn, err := connector.Connect()
	elapsed := time.Since(start)

	// Should fail
	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to connect to kubelet after")
	assert.Contains(t, err.Error(), "timeout: 2s")

	// Should have made multiple attempts (at least 4: 0s, 0.5s, 1s, 1.5s, 2s)
	attempts := atomic.LoadInt32(&attemptCount)
	assert.GreaterOrEqual(t, attempts, int32(4), "Should have made at least 4 attempts")
	assert.LessOrEqual(t, attempts, int32(6), "Should not have made too many attempts")

	// Should have taken approximately the timeout duration
	assert.GreaterOrEqual(t, elapsed, 2*time.Second, "Should have run for timeout duration")
	assert.Less(t, elapsed, 3*time.Second, "Should not have exceeded timeout by much")
}

// TestConnectorRetry_BackoffAdjustment tests that backoff is adjusted when approaching timeout.
func TestConnectorRetry_BackoffAdjustment(t *testing.T) {
	t.Parallel()
	var attemptCount int32
	var lastAttemptTime time.Time
	var attemptTimes []time.Time

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		lastAttemptTime = time.Now()
		attemptTimes = append(attemptTimes, lastAttemptTime)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	// Configure so that last backoff should be adjusted
	// Timeout: 3s, Backoff: 2s
	// Attempts at: 0s, 2s (elapsed=2s, remaining=1s, adjusted backoff=1s), 3s (timeout)
	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 3 * time.Second,
			InitBackoff: 2 * time.Second,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

	start := time.Now()
	conn, err := connector.Connect()
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, conn)

	// Should have made attempts at 0s, ~2s, and possibly ~3s (depending on timing)
	attempts := atomic.LoadInt32(&attemptCount)
	assert.GreaterOrEqual(t, attempts, int32(2), "Should have made at least 2 attempts")
	assert.LessOrEqual(t, attempts, int32(3), "Should have made at most 3 attempts")

	// Verify timing
	assert.GreaterOrEqual(t, elapsed, 3*time.Second, "Should respect timeout")
	assert.Less(t, elapsed, 4*time.Second, "Should not significantly exceed timeout")
}

// TestConnectorRetry_LegacyMode tests that initTimeout=0 disables retries.
func TestConnectorRetry_LegacyMode(t *testing.T) {
	t.Parallel()
	var attemptCount int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	// Legacy mode: InitTimeout = 0
	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 0, // Legacy mode - no retries!
			InitBackoff: 5 * time.Second,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

	start := time.Now()
	conn, err := connector.Connect()
	elapsed := time.Since(start)

	// Should fail immediately
	require.Error(t, err)
	assert.Nil(t, conn)

	// Should have made exactly 1 attempt (no retries)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attemptCount), "Should make exactly 1 attempt in legacy mode")

	// Should fail very quickly
	assert.Less(t, elapsed, 1*time.Second, "Should fail immediately without retries")
}

// TestConnectorRetry_AttemptCounting validates the attempt counting logic.
func TestConnectorRetry_AttemptCounting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		timeout     time.Duration
		backoff     time.Duration
		minAttempts int32
		maxAttempts int32
	}{
		{
			name:        "2s timeout / 500ms backoff",
			timeout:     2 * time.Second,
			backoff:     500 * time.Millisecond,
			minAttempts: 4, // 0s, 0.5s, 1s, 1.5s, 2s
			maxAttempts: 6,
		},
		{
			name:        "5s timeout / 1s backoff",
			timeout:     5 * time.Second,
			backoff:     1 * time.Second,
			minAttempts: 5, // 0s, 1s, 2s, 3s, 4s, 5s
			maxAttempts: 7,
		},
		{
			name:        "3s timeout / 2s backoff",
			timeout:     3 * time.Second,
			backoff:     2 * time.Second,
			minAttempts: 2, // 0s, 2s
			maxAttempts: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var attemptCount int32

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&attemptCount, 1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()

			node := createTestNodeWithPort(10250)
			k8sClient := fake.NewSimpleClientset(node)

			cfg := &config.Config{
				NodeName: "test-node",
				NodeIP:   "127.0.0.1",
				Kubelet: config.Kubelet{
					Enabled:     true,
					Port:        10250,
					Timeout:     5 * time.Second,
					InitTimeout: tt.timeout,
					InitBackoff: tt.backoff,
				},
			}

			restConfig := &rest.Config{
				Host: server.URL,
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

			_, err := connector.Connect()
			require.Error(t, err)

			attempts := atomic.LoadInt32(&attemptCount)
			assert.GreaterOrEqual(t, attempts, tt.minAttempts,
				"Should have made at least %d attempts, got %d", tt.minAttempts, attempts)
			assert.LessOrEqual(t, attempts, tt.maxAttempts,
				"Should have made at most %d attempts, got %d", tt.maxAttempts, attempts)

			t.Logf("%s: Made %d attempts (expected %d-%d)", tt.name, attempts, tt.minAttempts, tt.maxAttempts)
		})
	}
}

// Helper function to create test node with specific port.
func createTestNodeWithPort(port int32) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: corev1.NodeStatus{
			DaemonEndpoints: corev1.NodeDaemonEndpoints{
				KubeletEndpoint: corev1.DaemonEndpoint{
					Port: port,
				},
			},
		},
	}
}

// BenchmarkConnectorRetry_ImmediateSuccess benchmarks the overhead of retry logic when connection succeeds immediately
func BenchmarkConnectorRetry_ImmediateSuccess(b *testing.B) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 10 * time.Second,
			InitBackoff: 1 * time.Second,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		connector := DefaultConnector(k8sClient, cfg, restConfig, logger)
		conn, err := connector.Connect()
		if err != nil {
			b.Fatalf("Connection failed: %v", err)
		}
		if conn == nil {
			b.Fatal("Connection is nil")
		}
	}
}

// TestConnectorRetry_ErrorMessaging tests that error messages contain useful information.
func TestConnectorRetry_ErrorMessaging(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	node := createTestNodeWithPort(10250)
	k8sClient := fake.NewSimpleClientset(node)

	cfg := &config.Config{
		NodeName: "test-node",
		NodeIP:   "127.0.0.1",
		Kubelet: config.Kubelet{
			Enabled:     true,
			Port:        10250,
			Timeout:     5 * time.Second,
			InitTimeout: 1 * time.Second,
			InitBackoff: 200 * time.Millisecond,
		},
	}

	restConfig := &rest.Config{
		Host: server.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	connector := DefaultConnector(k8sClient, cfg, restConfig, logger)
	_, err := connector.Connect()

	require.Error(t, err)

	// Error should contain useful information
	errMsg := err.Error()
	assert.Contains(t, errMsg, "failed to connect to kubelet after")
	assert.Contains(t, errMsg, "attempts")
	assert.Contains(t, errMsg, "timeout: 1s")

	t.Logf("Error message: %s", errMsg)
}

// TestConnectorRetry_DifferentBackoffValues tests various backoff configurations.
func TestConnectorRetry_DifferentBackoffValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		backoff          time.Duration
		timeout          time.Duration
		expectedAttempts int32
	}{
		{"Fast backoff", 100 * time.Millisecond, 1 * time.Second, 10},
		{"Medium backoff", 500 * time.Millisecond, 2 * time.Second, 4},
		{"Slow backoff", 1 * time.Second, 3 * time.Second, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var attemptCount int32

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&attemptCount, 1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()

			node := createTestNodeWithPort(10250)
			k8sClient := fake.NewSimpleClientset(node)

			cfg := &config.Config{
				NodeName: "test-node",
				NodeIP:   "127.0.0.1",
				Kubelet: config.Kubelet{
					Enabled:     true,
					Port:        10250,
					Timeout:     5 * time.Second,
					InitTimeout: tt.timeout,
					InitBackoff: tt.backoff,
				},
			}

			restConfig := &rest.Config{
				Host: server.URL,
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			connector := DefaultConnector(k8sClient, cfg, restConfig, logger)

			start := time.Now()
			_, err := connector.Connect()
			elapsed := time.Since(start)

			require.Error(t, err)

			attempts := atomic.LoadInt32(&attemptCount)
			// Allow some tolerance (±2 attempts)
			assert.InDelta(t, tt.expectedAttempts, attempts, 2,
				fmt.Sprintf("Expected ~%d attempts, got %d", tt.expectedAttempts, attempts))

			// Timing should be close to timeout
			assert.InDelta(t, float64(tt.timeout), float64(elapsed), float64(1*time.Second),
				"Elapsed time should be close to timeout")

			t.Logf("%s: backoff=%v, timeout=%v → %d attempts in %v",
				tt.name, tt.backoff, tt.timeout, attempts, elapsed)
		})
	}
}
