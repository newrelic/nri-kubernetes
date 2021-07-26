package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

func TestDiscoverKSMWithPodLabel(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
		}}, nil)
	c.On("Config").Return(
		&rest.Config{BearerToken: "foobar"},
	)

	// and an Discoverer implementation
	d := podLabelDiscoverer{
		k8sClient:   c,
		logger:      logger,
		ksmPodLabel: "custom_ksm_3",
	}

	// When discovering the KSM client
	ksmClient, err := d.Discover(timeout)

	// The call works correctly
	assert.Nil(t, err, "should not return error")
	// And we get the Pod we expect
	assert.Equal(t, "4.3.2.1", ksmClient.(*ksm).nodeIP)
}

func TestPodLabelDiscovererSelectsSamePod(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.2"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.3"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.6"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.4"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.5"}},
		}}, nil)
	c.On("Config").Return(
		&rest.Config{BearerToken: "foobar"},
	)

	// and an Discoverer implementation
	d := podLabelDiscoverer{
		k8sClient:   c,
		logger:      logger,
		ksmPodLabel: "custom_ksm_3",
	}

	// When discovering the KSM client
	pod, err := d.Discover(timeout)

	// The call works correctly
	assert.Nil(t, err, "should not return error")

	// And the returned Pod is the one with the alphabetically-sorted "highest" ip
	assert.Equal(t, "4.3.2.6", pod.NodeIP())
}

func podLabelDiscovererConfig() PodLabelDiscovererConfig {
	return PodLabelDiscovererConfig{
		K8sClient:   new(client.MockedKubernetes),
		Logger:      logger,
		KSMPodLabel: "custom_ksm_3",
		KSMPodPort:  1234,
		KSMScheme:   "http",
	}
}

func Test_PodLabelDiscoverer_requires(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*PodLabelDiscovererConfig){
		"logger":            func(c *PodLabelDiscovererConfig) { c.Logger = nil },
		"kubernetes_client": func(c *PodLabelDiscovererConfig) { c.K8sClient = nil },
		"KSM_pod_port":      func(c *PodLabelDiscovererConfig) { c.KSMPodPort = 0 },
		"KSM_pod_label":     func(c *PodLabelDiscovererConfig) { c.KSMPodLabel = "" },
		"KSM_scheme":        func(c *PodLabelDiscovererConfig) { c.KSMScheme = "" },
	}

	for caseName, mutateF := range cases {
		mutateF := mutateF

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			config := podLabelDiscovererConfig()

			mutateF(&config)

			d, err := NewPodLabelDiscoverer(config)
			require.NotNil(t, err)
			require.Nil(t, d)
		})
	}
}

func Test_PodLabelDiscoverer_validates_configured_scheme_by(t *testing.T) {
	t.Parallel()

	t.Run("accepting_http_scheme", func(t *testing.T) {
		t.Parallel()

		config := podLabelDiscovererConfig()
		d, err := NewPodLabelDiscoverer(config)
		require.Nil(t, err)
		require.NotNil(t, d)
	})

	t.Run("accepting_https_scheme", func(t *testing.T) {
		t.Parallel()

		config := podLabelDiscovererConfig()
		config.KSMScheme = "https"

		d, err := NewPodLabelDiscoverer(config)
		require.Nil(t, err)
		require.NotNil(t, d)
	})

	t.Run("rejecting_unsupported_scheme", func(t *testing.T) {
		t.Parallel()

		config := podLabelDiscovererConfig()
		config.KSMScheme = "foo"
		d, err := NewPodLabelDiscoverer(config)
		require.NotNil(t, err)
		require.Nil(t, d)
	})
}
