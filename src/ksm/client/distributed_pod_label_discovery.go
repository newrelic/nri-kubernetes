package client

import (
	"fmt"
	"net/url"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

type distributedPodLabelDiscoverer struct {
	ksmPodLabel  string
	ksmNamespace string
	ownNodeIP    string
	logger       log.Logger
	k8sClient    client.Kubernetes
}

func (p *distributedPodLabelDiscoverer) findAllLabeledPodsRunningOnNode() ([]v1.Pod, error) {
	pods, err := p.k8sClient.FindPodsByLabel(p.ksmNamespace, metav1.LabelSelector{
		MatchLabels: map[string]string{
			p.ksmPodLabel: "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying API server for pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("discovering KSM with label %q in namespace %q: %w", p.ksmPodLabel, p.ksmNamespace, errNoKSMPodsFound)
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
		)
		clients = append(clients, ksmClient)
	}
	return clients, nil
}

// DistributedPodLabelDiscovererConfig stores configuration for DistributedPodLabelDiscoverer.
type DistributedPodLabelDiscovererConfig struct {
	KSMPodLabel  string
	NodeIP       string
	KSMNamespace string
	K8sClient    client.Kubernetes
	Logger       log.Logger
}

// NewDistributedPodLabelDiscoverer creates a new KSM discoverer that will find KSM pods using k8s labels.
func NewDistributedPodLabelDiscoverer(config DistributedPodLabelDiscovererConfig) (client.MultiDiscoverer, error) {
	if config.Logger == nil {
		return nil, fmt.Errorf("logger must be set")
	}

	if config.KSMPodLabel == "" {
		return nil, fmt.Errorf("KSM pod label can't be empty")
	}

	if config.K8sClient == nil {
		return nil, fmt.Errorf("Kubernetes client must be set")
	}

	if config.NodeIP == "" {
		return nil, fmt.Errorf("node IP can't be empty")
	}

	return &distributedPodLabelDiscoverer{
		logger:       config.Logger,
		ownNodeIP:    config.NodeIP,
		k8sClient:    config.K8sClient,
		ksmPodLabel:  config.KSMPodLabel,
		ksmNamespace: config.KSMNamespace,
	}, nil
}
