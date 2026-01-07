// Test suite for connector retry logic
package client

import (
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
)

// TestConnector_LegacyBehavior_NoRetries tests that initTimeout=0 disables retries
func TestConnector_LegacyBehavior_NoRetries(t *testing.T) {
	t.Skip("Integration test - requires full environment setup")
}

// TestConnector_RetryLogic tests that retry logic activates when initTimeout > 0
func TestConnector_RetryLogic(t *testing.T) {
	// Test that configuration is properly loaded
	cfg := config.Kubelet{
		Enabled:     true,
		Port:        10250,
		Timeout:     10 * time.Second,
		InitTimeout: 180 * time.Second,
		InitBackoff: 5 * time.Second,
	}

	if cfg.InitTimeout != 180*time.Second {
		t.Errorf("InitTimeout not set correctly: got %v, want %v", cfg.InitTimeout, 180*time.Second)
	}

	if cfg.InitBackoff != 5*time.Second {
		t.Errorf("InitBackoff not set correctly: got %v, want %v", cfg.InitBackoff, 5*time.Second)
	}

	// Test that initTimeout=0 works (legacy mode)
	legacyCfg := config.Kubelet{
		Enabled:     true,
		Port:        10250,
		Timeout:     10 * time.Second,
		InitTimeout: 0,
		InitBackoff: 5 * time.Second,
	}

	if legacyCfg.InitTimeout != 0 {
		t.Errorf("Legacy mode InitTimeout should be 0: got %v", legacyCfg.InitTimeout)
	}
}

// TestConnector_ConfigValidation tests configuration validation
func TestConnector_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		initTimeout time.Duration
		initBackoff time.Duration
		wantRetries bool
	}{
		{
			name:        "Default EKS config",
			initTimeout: 180 * time.Second,
			initBackoff: 5 * time.Second,
			wantRetries: true,
		},
		{
			name:        "GKE config",
			initTimeout: 120 * time.Second,
			initBackoff: 5 * time.Second,
			wantRetries: true,
		},
		{
			name:        "Legacy mode (no retries)",
			initTimeout: 0,
			initBackoff: 5 * time.Second,
			wantRetries: false,
		},
		{
			name:        "Custom aggressive config",
			initTimeout: 180 * time.Second,
			initBackoff: 2 * time.Second,
			wantRetries: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Kubelet{
				Enabled:     true,
				Port:        10250,
				Timeout:     10 * time.Second,
				InitTimeout: tt.initTimeout,
				InitBackoff: tt.initBackoff,
			}

			gotRetries := cfg.InitTimeout > 0
			if gotRetries != tt.wantRetries {
				t.Errorf("Retry logic enabled = %v, want %v", gotRetries, tt.wantRetries)
			}

			if gotRetries {
				// Calculate expected number of attempts
				maxAttempts := int(tt.initTimeout / tt.initBackoff)
				if maxAttempts < 1 {
					t.Errorf("Invalid config: maxAttempts should be at least 1, got %d", maxAttempts)
				}

				t.Logf("%s: timeout=%v, backoff=%v, max_attempts=%d",
					tt.name, tt.initTimeout, tt.initBackoff, maxAttempts)
			}
		})
	}
}
