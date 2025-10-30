package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

func TestSetupKubelet(t *testing.T) {
	c := config.Config{
		Verbose: false,
		Kubelet: config.Kubelet{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]interface{}{
				"newrelic.com/scrape": "true",
			},
		},
		Interval: 10 * time.Second,
	}
	logger = logutil.Discard
	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers, namespaceCache)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
	assert.NotEmpty(t, scraper.Filterer)
}

func TestSetupKSM(t *testing.T) {
	c := config.Config{
		Verbose: false,
		KSM:     config.KSM{},
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]interface{}{
				"newrelic.com/scrape": "true",
			},
		},
		Interval: 10 * time.Second,
	}
	logger = logutil.Discard
	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKSM(&c, &providers, namespaceCache)
	assert.NoError(t, err)
	assert.NotEmpty(t, scraper)
	assert.NotEmpty(t, scraper.Filterer)
}

//nolint:paralleltest // timing test should not run in parallel
func TestMeasureTime(t *testing.T) {
	// Test that measureTime accurately measures function execution time
	expectedDuration := 100 * time.Millisecond

	duration := measureTime(func() {
		time.Sleep(expectedDuration)
	})

	// Allow for some tolerance in timing (10ms)
	tolerance := 10 * time.Millisecond
	assert.True(t, duration >= expectedDuration, "measured duration should be at least the expected duration")
	assert.True(t, duration < expectedDuration+tolerance, "measured duration should be close to expected duration")
}

//nolint:paralleltest // timing test should not run in parallel
func TestMeasureTimeWithZeroDuration(t *testing.T) {
	// Test that measureTime works with instant functions
	duration := measureTime(func() {
		// Do nothing
	})

	assert.True(t, duration >= 0, "duration should be non-negative")
	assert.True(t, duration < 10*time.Millisecond, "duration should be very small for empty function")
}
