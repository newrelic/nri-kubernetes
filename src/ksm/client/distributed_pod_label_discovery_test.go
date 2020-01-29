package client

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

func TestDistributedDiscoverKSMWithPodLabel(t *testing.T) {
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
			{Status: v1.PodStatus{HostIP: "4.3.2.2"}},
		}}, nil)

	d := distributedPodLabelDiscoverer{
		k8sClient:   c,
		logger:      logger,
		ownNodeIP:   "4.3.2.1",
		ksmPodLabel: "custom_ksm_3",
	}

	ksmClients, err := d.Discover(timeout)

	assert.NoError(t, err)
	assert.Len(t, ksmClients, 2)
	for _, ksmClient := range ksmClients {
		assert.Equal(t, "4.3.2.1", ksmClient.(*ksm).nodeIP)
	}
}
