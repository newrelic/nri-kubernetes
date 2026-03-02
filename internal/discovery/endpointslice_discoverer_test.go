package discovery_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

//nolint:ireturn // Returning interface is correct for test helper.
func newTestEndpointSliceDiscoverer(t *testing.T, config discovery.EndpointsDiscoveryConfig) discovery.EndpointsDiscoverer {
	t.Helper()
	discoverer, stopCh, err := discovery.NewEndpointSliceDiscoverer(config)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh) })
	return discoverer
}

func Test_endpointslice_discoverer_creation_fails_when_no_client_is_provided(t *testing.T) {
	t.Parallel()

	_, _, err := discovery.NewEndpointSliceDiscoverer(discovery.EndpointsDiscoveryConfig{})
	assert.Error(t, err, "error expected since client is nil")
}

func Test_endpointslice_discoverer_basic_functionality(t *testing.T) {
	t.Parallel()

	// GIVEN: A single EndpointSlice with 2 ready endpoints
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-state-metrics-abc123",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"kubernetes.io/service-name": "kube-state-metrics",
				"selector":                   "matching",
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
			{
				Addresses: []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     ptr.To("http-metrics"),
				Port:     ptr.To(int32(8080)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{
		Client: client,
	}

	// WHEN: Creating discoverer and calling Discover()
	discoverer := newTestEndpointSliceDiscoverer(t, config)

	hosts, err := discoverer.Discover()

	// THEN: Should return both endpoints in sorted order
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.2:8080"}, hosts)
}

func Test_endpointslice_discoverer_filters_not_ready_endpoints(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice with mix of ready and not-ready endpoints
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
			Labels:    map[string]string{"selector": "matching"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
			{
				Addresses: []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(false), // Not ready
				},
			},
			{
				Addresses: []string{"10.0.0.3"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     ptr.To(int32(8080)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	// WHEN: Discover() is called
	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	// THEN: Only ready endpoints should be returned
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.3:8080"}, hosts)
	assert.NotContains(t, hosts, "10.0.0.2:8080", "Not-ready endpoint should be filtered out")
}

func Test_endpointslice_discoverer_handles_nil_ready_condition(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice with nil Ready condition
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: nil, // Nil ready condition
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     ptr.To(int32(8080)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	// WHEN: Discover() is called
	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	// THEN: Should handle gracefully without panic and include endpoint
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080"}, hosts)
}

func Test_endpointslice_discoverer_handles_nil_port(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice with nil Port
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
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
				Port:     nil, // Nil port
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Empty(t, hosts, "Endpoints with nil port should be skipped")
}

func Test_endpointslice_discoverer_multiple_slices(t *testing.T) {
	t.Parallel()

	// GIVEN: Multiple EndpointSlice objects for same service
	slice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-state-metrics-1",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"kubernetes.io/service-name": "kube-state-metrics",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	slice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-state-metrics-2",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"kubernetes.io/service-name": "kube-state-metrics",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	client := testclient.NewSimpleClientset(slice1, slice2) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.2:8080"}, hosts)
}

func Test_endpointslice_discoverer_deduplication(t *testing.T) {
	t.Parallel()

	// GIVEN: Multiple slices with duplicate host:port combinations
	slice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-1",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	slice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-2",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"}, // Duplicate IP
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)}, // Same port
		},
	}

	client := testclient.NewSimpleClientset(slice1, slice2) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080"}, hosts)
	assert.Len(t, hosts, 1, "Duplicate endpoint should be deduplicated")
}

func Test_endpointslice_discoverer_empty_results(t *testing.T) {
	t.Parallel()

	// GIVEN: No matching EndpointSlices
	client := testclient.NewSimpleClientset() //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	// WHEN: Discover() is called
	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	// THEN: Should return empty slice without error
	require.NoError(t, err)
	assert.Empty(t, hosts)
}

func Test_endpointslice_discoverer_sorting(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice with multiple endpoints in random order
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.9"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
			{
				Addresses:  []string{"10.0.0.5"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	// WHEN: Discover() is called
	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	// THEN: Results should be sorted alphabetically
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.5:8080", "10.0.0.9:8080"}, hosts)
}

type endpointSliceTestData struct {
	configModifier func(s *discovery.EndpointsDiscoveryConfig)
	result         []string
}

func Test_endpointslice_discovery_with_filters(t *testing.T) {
	t.Parallel()

	// Create test EndpointSlices matching the structure from legacy Endpoints tests
	client := testclient.NewSimpleClientset(getFirstEndpointSlice(), getSecondEndpointSlice()) //nolint:staticcheck // Deprecated but no alternative.

	testCases := map[string]endpointSliceTestData{
		"not_matching_selector": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.LabelSelector = "not-matching"
			},
			result: nil,
		},
		"matching_selector": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.LabelSelector = "selector=matching"
			},
			result: []string{"1.2.3.4:80", "5.6.7.8:81"},
		},
		"no_selector_no_namespace_no_port": {
			configModifier: func(_ *discovery.EndpointsDiscoveryConfig) {
			},
			result: []string{"1.2.3.4:80", "5.6.7.8:81"},
		},
		"not_matching_namespace": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.Namespace = "different-namespace"
			},
			result: nil,
		},
		"matching_namespace": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.Namespace = "testNamespace2"
			},
			result: []string{"5.6.7.8:81"},
		},
		"not_matching_port": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.Port = 1000
			},
			result: nil,
		},
		"matching_port": {
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
				s.Port = 81
			},
			result: []string{"5.6.7.8:81"},
		},
	}

	for testName, testData := range testCases {
		testData := testData

		c := discovery.EndpointsDiscoveryConfig{
			Client: client,
		}

		testData.configModifier(&c)

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			d := newTestEndpointSliceDiscoverer(t, c)

			e, err := d.Discover()
			require.NoError(t, err)

			assert.Equal(t, testData.result, e)
		})
	}
}

func Test_endpointslice_discoverer_multiple_addresses_per_endpoint(t *testing.T) {
	t.Parallel()

	// GIVEN: Endpoint with multiple addresses (rare but valid in EndpointSlice)
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1", "10.0.0.2"}, // Multiple addresses
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	// WHEN: Discover() is called
	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	// THEN: All addresses should be returned
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.2:8080"}, hosts)
}

func Test_endpointslice_discoverer_multiple_ports(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice with multiple ports
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
			{Port: ptr.To(int32(9090)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{Client: client}

	discoverer := newTestEndpointSliceDiscoverer(t, config)
	hosts, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.1:9090"}, hosts)
}

func getFirstEndpointSlice() *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-1",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"selector": "matching",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"1.2.3.4"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     ptr.To(int32(80)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}

func getSecondEndpointSlice() *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice-2",
			Namespace: "testNamespace2",
			Labels: map[string]string{
				"selector": "matching",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"5.6.7.8"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     ptr.To(int32(81)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		},
	}
}

func Test_endpointslice_discoverer_with_timeout_wrapper(t *testing.T) {
	t.Parallel()

	// GIVEN: EndpointSlice that will be discovered
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "testNamespace",
			Labels:    map[string]string{"app": "test"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: ptr.To(int32(8080)), Protocol: ptr.To(corev1.ProtocolTCP)},
		},
	}

	client := testclient.NewSimpleClientset(slice) //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{
		Client:        client,
		LabelSelector: "app=test",
	}

	innerDiscoverer, stopCh, err := discovery.NewEndpointSliceDiscoverer(config)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh) })

	timeoutDiscoverer := &discovery.EndpointsDiscovererWithTimeout{
		EndpointsDiscoverer: innerDiscoverer,
		BackoffDelay:        100 * time.Millisecond,
		Timeout:             2 * time.Second,
	}

	endpoints, err := timeoutDiscoverer.Discover()
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.1:8080"}, endpoints)
}

func Test_endpointslice_discoverer_timeout_on_empty_results(t *testing.T) {
	t.Parallel()

	// GIVEN: No matching EndpointSlices (will return empty results)
	client := testclient.NewSimpleClientset() //nolint:staticcheck // Deprecated but no alternative.
	config := discovery.EndpointsDiscoveryConfig{
		Client:        client,
		LabelSelector: "app=nonexistent",
	}

	innerDiscoverer, stopCh, err := discovery.NewEndpointSliceDiscoverer(config)
	require.NoError(t, err)
	t.Cleanup(func() { close(stopCh) })

	timeoutDiscoverer := &discovery.EndpointsDiscovererWithTimeout{
		EndpointsDiscoverer: innerDiscoverer,
		BackoffDelay:        50 * time.Millisecond,
		Timeout:             150 * time.Millisecond,
	}

	_, err = timeoutDiscoverer.Discover()
	assert.ErrorIs(t, err, discovery.ErrDiscoveryTimeout)
}
