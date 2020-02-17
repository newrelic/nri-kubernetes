package controlplane

import (
	"net/url"
	"testing"

	"github.com/newrelic/nri-kubernetes/src/metric"
	"github.com/stretchr/testify/assert"
)

func TestSetEtcdTLSComponentOption(t *testing.T) {
	// first assert that ETCD has no TLS options by default
	components := BuildComponentList()
	etcd := findComponentByName(Etcd, components)

	assert.Equal(t, "", etcd.TLSSecretName)
	assert.Equal(t, "", etcd.TLSSecretNamespace)
	assert.True(t, etcd.Skip)

	// now set the TLS Configuration, and assert they are properly set
	const (
		tlsSecretName      = "my-secret-name"
		tlsSecretNamespace = "iluvtests"
	)

	components = BuildComponentList(WithEtcdTLSConfig(tlsSecretName, tlsSecretNamespace))
	etcd = findComponentByName(Etcd, components)

	assert.Equal(t, tlsSecretName, etcd.TLSSecretName)
	assert.Equal(t, tlsSecretNamespace, etcd.TLSSecretNamespace)
	assert.False(t, etcd.Skip)

}

func TestWithEndpointURL(t *testing.T) {
	var testCases = []struct {
		name                string
		components          []Component
		assertShouldPanic   func()
		assertShouldSucceed func(Component, string)
		endpointURL         string
	}{
		{
			name:       "It should panic when component not found",
			components: []Component{},
			assertShouldPanic: func() {
				assert.True(t, true)
			},
			assertShouldSucceed: func(component Component, endpointURL string) {
				assert.Fail(t, "WithEndpointURL should have panic'ed!")
			},
			endpointURL: "https://localhost:12344",
		},
		{
			name: "It should panic with invalid URL",
			components: []Component{
				{
					Name: Etcd,
					Labels: []labels{
						{"k8s-app": "etcd-manager-main"},
						{"tier": "control-plane", "component": "etcd"},
					},
					Queries: metric.EtcdQueries,
					Specs:   metric.EtcdSpecs,
					Endpoint: url.URL{
						Scheme: "https",
						Host:   "127.0.0.1:4001",
					},
				},
			},
			assertShouldPanic: func() {
				assert.True(t, true)
			},
			assertShouldSucceed: func(component Component, endpointURL string) {
				assert.Fail(t, "WithEndpointURL should have panic'ed!")
			},
			endpointURL: "\x00\x01\x02",
		},
		{
			name: "It should set endpoint URL and no auth with valid http URL",
			components: []Component{
				{
					Name: Etcd,
					Labels: []labels{
						{"k8s-app": "etcd-manager-main"},
						{"tier": "control-plane", "component": "etcd"},
					},
					Queries: metric.EtcdQueries,
					Specs:   metric.EtcdSpecs,
					Endpoint: url.URL{
						Scheme: "https",
						Host:   "127.0.0.1:4001",
					},
				},
			},
			assertShouldPanic: func() {
				assert.Fail(t, "WithEndpointURL should not have panic'ed!")
			},
			assertShouldSucceed: func(component Component, endpointURL string) {
				assert.Equal(t, endpointURL, component.Endpoint.String())
				assert.False(t, component.UseServiceAccountAuthentication)
			},
			endpointURL: "http://localhost:8080",
		},
		{
			name: "It should set endpoint URL and service account auth with valid https URL",
			components: []Component{
				{
					Name: Etcd,
					Labels: []labels{
						{"k8s-app": "etcd-manager-main"},
						{"tier": "control-plane", "component": "etcd"},
					},
					Queries: metric.EtcdQueries,
					Specs:   metric.EtcdSpecs,
					Endpoint: url.URL{
						Scheme: "https",
						Host:   "127.0.0.1:4001",
					},
				},
			},
			assertShouldPanic: func() {
				assert.Fail(t, "WithEndpointURL should not have panic'ed!")
			},
			assertShouldSucceed: func(component Component, endpointURL string) {
				assert.Equal(t, endpointURL, component.Endpoint.String())
				assert.True(t, component.UseServiceAccountAuthentication)
			},
			endpointURL: "https://localhost:8080",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given a deferred panic handler
			defer func() {
				if r := recover(); r != nil {
					testCase.assertShouldPanic()
				}
			}()

			// Given a component name
			componentName := Etcd

			// When executing the WithEndpointURL option
			WithEndpointURL(componentName, testCase.endpointURL)(testCase.components)

			// The call should succeed or fail (see above)
			testCase.assertShouldSucceed(testCase.components[0], testCase.endpointURL)
		})
	}
}
