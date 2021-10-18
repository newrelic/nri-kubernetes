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

type podLabelDiscoverer struct {
	ksmPodLabel  string
	ksmNamespace string
	logger       log.Logger
	k8sClient    client.Kubernetes
	ksmPodPort   int
	ksmScheme    string
}

func (p *podLabelDiscoverer) findSingleKSMPodByLabel() (*v1.Pod, error) {
	pods, err := p.k8sClient.FindPodsByLabel(p.ksmNamespace, metav1.LabelSelector{
		MatchLabels: map[string]string{
			p.ksmPodLabel: "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying API server for Pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("discovering KSM with label %q in namespace %q: %w", p.ksmPodLabel, p.ksmNamespace, errNoKSMPodsFound)
	}

	// In case there are multiple pods, we must be be sure to deterministically select the same Pod on each node
	// So we chose, for example, the HostIp with highest precedence in alphabetical order
	var chosenPod v1.Pod
	for _, pod := range pods.Items {

		if pod.Status.HostIP == "" {
			continue
		}

		if chosenPod.Status.HostIP == "" || pod.Status.HostIP > chosenPod.Status.HostIP {
			chosenPod = pod
		}
	}

	return &chosenPod, nil
}

// Discover will find a single KSM pod using the provided label.
func (p *podLabelDiscoverer) Discover(timeout time.Duration) (client.HTTPClient, error) {
	pod, err := p.findSingleKSMPodByLabel()
	if err != nil {
		return nil, err
	}

	endpoint := url.URL{
		Scheme: p.ksmScheme,
		Host:   fmt.Sprintf("%s:%d", pod.Status.PodIP, p.ksmPodPort),
	}

	ksmClient := newKSMClient(
		timeout,
		pod.Status.HostIP,
		endpoint,
		p.logger,
	)
	return ksmClient, nil
}

// PodLabelDiscovererConfig holds KSM PodLabelDiscoverer configuration.
type PodLabelDiscovererConfig struct {
	KSMPodLabel  string
	KSMPodPort   int
	KSMScheme    string
	KSMNamespace string
	Logger       log.Logger
	K8sClient    client.Kubernetes
}

// NewPodLabelDiscoverer creates a new KSM discoverer that will find KSM pods using k8s labels.
func NewPodLabelDiscoverer(config PodLabelDiscovererConfig) (client.Discoverer, error) {
	if config.Logger == nil {
		return nil, fmt.Errorf("logger must be set")
	}

	if config.KSMPodLabel == "" {
		return nil, fmt.Errorf("KSM pod label can't be empty")
	}

	if config.KSMPodPort == 0 {
		return nil, fmt.Errorf("KSM pod port can't be zero")
	}

	if config.K8sClient == nil {
		return nil, fmt.Errorf("Kubernetes client must be set")
	}

	if config.KSMScheme == "" {
		return nil, fmt.Errorf("KMS scheme can't be empty")
	}

	if config.KSMScheme != "" && config.KSMScheme != "https" && config.KSMScheme != "http" {
		return nil, fmt.Errorf("unsupported KSM scheme. Expected 'http' or 'https', got %q", config.KSMScheme)
	}

	return &podLabelDiscoverer{
		logger:       config.Logger,
		k8sClient:    config.K8sClient,
		ksmPodLabel:  config.KSMPodLabel,
		ksmPodPort:   config.KSMPodPort,
		ksmScheme:    config.KSMScheme,
		ksmNamespace: config.KSMNamespace,
	}, nil
}
