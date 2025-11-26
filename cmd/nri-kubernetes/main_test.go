package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	kubeletMetric "github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
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
	interfaceCache := kubeletMetric.NewInterfaceCache()
	providers := clusterClients{
		k8s: fake.NewSimpleClientset(),
	}
	scraper, err := setupKubelet(&c, &providers, namespaceCache, interfaceCache)
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

func TestInterfaceCacheVacuumInterval(t *testing.T) {
	t.Parallel()

	// Test that vacuum only occurs at expected intervals
	vacuumOccurrences := 0

	// Simulate 50 scrapes
	for scrapeCount := 1; scrapeCount <= 50; scrapeCount++ {
		if scrapeCount%interfaceCacheVacuumInterval == 0 {
			vacuumOccurrences++
		}
	}

	// With interval of 10, we expect 5 vacuums in 50 scrapes (at 10, 20, 30, 40, 50)
	expectedVacuums := 50 / interfaceCacheVacuumInterval
	assert.Equal(t, expectedVacuums, vacuumOccurrences, "vacuum should occur every %d scrapes", interfaceCacheVacuumInterval)

	// Verify first vacuum happens at the expected scrape
	assert.NotEqual(t, 0, 1%interfaceCacheVacuumInterval, "should NOT vacuum on first scrape")
	assert.Equal(t, 0, 10%interfaceCacheVacuumInterval, "should vacuum on 10th scrape")
}
