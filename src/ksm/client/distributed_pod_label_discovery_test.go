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

func TestDistributedDiscoverKSMWithPodLabel(t *testing.T) {
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.2"}},
		}}, nil)
	c.On("Config").Return(
		&rest.Config{BearerToken: "foobar"},
	)

	d, err := NewDistributedPodLabelDiscoverer(DistributedPodLabelDiscovererConfig{
		K8sClient:   c,
		Logger:      logger,
		NodeIP:      "4.3.2.1",
		KSMPodLabel: "custom_ksm_3",
	})
	require.Nil(t, err)

	ksmClients, err := d.Discover(timeout)

	assert.NoError(t, err)
	assert.Len(t, ksmClients, 2)
	for _, ksmClient := range ksmClients {
		assert.Equal(t, "4.3.2.1", ksmClient.(*ksm).nodeIP)
	}
}

func distributedPodLabelDiscovererConfig() DistributedPodLabelDiscovererConfig {
	return DistributedPodLabelDiscovererConfig{
		K8sClient:   new(client.MockedKubernetes),
		Logger:      logger,
		NodeIP:      "4.3.2.1",
		KSMPodLabel: "custom_ksm_3",
	}
}

func Test_DistributedPodLabelDiscoverer_requires(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*DistributedPodLabelDiscovererConfig){
		"logger":            func(c *DistributedPodLabelDiscovererConfig) { c.Logger = nil },
		"kubernetes_client": func(c *DistributedPodLabelDiscovererConfig) { c.K8sClient = nil },
		"node_IP":           func(c *DistributedPodLabelDiscovererConfig) { c.NodeIP = "" },
		"KSM_pod_label":     func(c *DistributedPodLabelDiscovererConfig) { c.KSMPodLabel = "" },
	}

	for caseName, mutateF := range cases {
		mutateF := mutateF

		t.Run(caseName, func(t *testing.T) {
			t.Parallel()

			config := distributedPodLabelDiscovererConfig()

			mutateF(&config)

			d, err := NewDistributedPodLabelDiscoverer(config)
			require.NotNil(t, err)
			require.Nil(t, d)
		})
	}
}
