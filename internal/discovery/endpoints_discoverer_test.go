package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

func Test_endpoint_discoverer_creation_fails_when_no_client_is_provided(t *testing.T) {
	t.Parallel()

	_, err := discovery.NewEndpointsDiscoverer(discovery.EndpointsDiscoveryConfig{})
	assert.Error(t, err, "error expected since client is nil")
}

type testData struct {
	configModifier func(s *discovery.EndpointsDiscoveryConfig)
	result         []string
}

func Test_endpoints_discovery_with(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(getFirstEndpoints(), getSecondEndpoints())

	testCases := map[string]testData{
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
			configModifier: func(s *discovery.EndpointsDiscoveryConfig) {
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

			d, err := discovery.NewEndpointsDiscoverer(c)
			require.NoError(t, err)

			e, err := d.Discover()
			require.NoError(t, err)

			assert.Equal(t, testData.result, e)
		})
	}
}

func getFirstEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testNamespace",
			Labels: map[string]string{
				"selector": "matching",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "1.2.3.4",
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port: 80,
					},
				},
			},
		},
	}
}

func getSecondEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testNamespace2",
			Labels: map[string]string{
				"selector": "matching",
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "5.6.7.8",
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port: 81,
					},
				},
			},
		},
	}
}
