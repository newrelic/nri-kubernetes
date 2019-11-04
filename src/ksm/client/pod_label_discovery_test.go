package client

import (
	"testing"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

func TestDiscoverKSMWithPodLabel(t *testing.T) {
	// Given a client
	c := new(client.MockedKubernetes)
	c.On("FindPodsByLabel", mock.Anything, mock.Anything).
		Return(&v1.PodList{Items: []v1.Pod{
			{Status: v1.PodStatus{HostIP: "4.3.2.1"}},
		}}, nil)

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
