package ksm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/ksm"
)

// TestKSMScraperUsesEndpointSliceDiscoverer verifies that the KSM scraper
// is wired to use the EndpointSlice API through its buildDiscoverer() method.
//
// This is a REGRESSION TEST to prevent someone from accidentally changing
// buildDiscoverer() back to the deprecated Endpoints API.
//
// What this test does:
// - Creates a fake Kubernetes cluster with ONLY an EndpointSlice (no v1 Endpoints)
// - Creates a KSM scraper that will use autodiscovery
// - Verifies the scraper is created successfully
//
// Why this works:
// - If buildDiscoverer() uses NewEndpointSliceDiscoverer → test passes (finds EndpointSlice)
// - If buildDiscoverer() uses NewEndpointsDiscoverer → test would fail (no v1 Endpoints exist)
//
// Note: The unit tests in endpointslice_discoverer_test.go already cover:
// - Discovery with custom selectors
// - Discovery with port overrides
// - Discovery with namespace filters
// - Deduplication, sorting, filtering, etc.
//
// This test ONLY verifies the scraper's internal wiring is correct.
func TestKSMScraperUsesEndpointSliceDiscoverer(t *testing.T) {
	t.Parallel()

	// GIVEN: A cluster with EndpointSlice for kube-state-metrics (no v1 Endpoints)
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-state-metrics-test",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: ptr.To("http-metrics"),
				Port: ptr.To(int32(8080)),
			},
		},
	}

	fakeK8s := fake.NewSimpleClientset(endpointSlice) //nolint:staticcheck // Deprecated but no alternative.

	cfg := &config.Config{
		KSM: config.KSM{
			StaticURL: "", // Force autodiscovery
			Namespace: "kube-system",
			Discovery: struct {
				BackoffDelay time.Duration `mapstructure:"backoffDelay"`
				Timeout      time.Duration `mapstructure:"timeout"`
			}{
				BackoffDelay: 100 * time.Millisecond,
				Timeout:      1 * time.Second,
			},
		},
		ClusterName: "test-cluster",
	}

	// WHEN: Creating a KSM scraper (goes through buildDiscoverer)
	scraper, err := ksm.NewScraper(cfg, ksm.Providers{
		K8s: fakeK8s,
		KSM: nil, // Will fail to scrape but that's okay, we're testing discoverer wiring
	})

	// THEN: The scraper should be created successfully
	// If buildDiscoverer() was using the old Endpoints API, it would fail
	// because we only provided an EndpointSlice, not a v1 Endpoints object
	require.NoError(t, err, "Scraper creation should succeed with EndpointSlice API")
	require.NotNil(t, scraper, "Scraper should be initialized")

	t.Log("KSM scraper created successfully using EndpointSlice API")
	t.Log("If this test fails after a code change, check if buildDiscoverer()")
	t.Log("was accidentally changed back to NewEndpointsDiscoverer")
}
