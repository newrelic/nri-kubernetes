package client

import (
	"fmt"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

type distributedPodLabelDiscoverer struct {
	ksmPodLabel string
	ownNodeIP   string
	logger      *logrus.Logger
	k8sClient   client.Kubernetes
}

func (p *distributedPodLabelDiscoverer) findAllLabeledPodsRunningOnNode() ([]v1.Pod, error) {
	pods, err := p.k8sClient.FindPodsByLabel(p.ksmPodLabel, "true")
	if err != nil {
		return nil, fmt.Errorf("querying API server for pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("discovering KSM with label %q: %w", p.ksmPodLabel, errNoKSMPodsFound)
	}

	var foundPods []v1.Pod
	for _, pod := range pods.Items {
		if pod.Status.HostIP == "" {
			continue
		}

		if pod.Status.HostIP == p.ownNodeIP {
			p.logger.Debugf("Found KSM pod running on this node, pod IP: %s", pod.Status.PodIP)
			foundPods = append(foundPods, pod)
		}
	}

	return foundPods, nil
}

// Discover will find all KSM pods in the current node using the provided label.
func (p *distributedPodLabelDiscoverer) Discover(timeout time.Duration) ([]client.HTTPClient, error) {
	pods, err := p.findAllLabeledPodsRunningOnNode()
	if err != nil {
		return nil, err
	}

	var clients []client.HTTPClient
	for _, pod := range pods {
		endpoint := url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:8080", pod.Status.PodIP),
		}
		ksmClient := newKSMClient(
			timeout,
			pod.Status.HostIP,
			endpoint,
			p.logger,
			p.k8sClient,
		)
		clients = append(clients, ksmClient)
	}
	return clients, nil
}

// NewDistributedPodLabelDiscoverer creates a new KSM discoverer that will find KSM pods using k8s labels
func NewDistributedPodLabelDiscoverer(ksmPodLabel string, nodeIP string, logger *logrus.Logger, k8sClient client.Kubernetes) client.MultiDiscoverer {
	return &distributedPodLabelDiscoverer{
		logger:      logger,
		ownNodeIP:   nodeIP,
		k8sClient:   k8sClient,
		ksmPodLabel: ksmPodLabel,
	}
}
