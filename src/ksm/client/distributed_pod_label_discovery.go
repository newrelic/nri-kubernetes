package client

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/src/client"
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
		return nil, errors.Wrap(err, "could not query api server for pods")
	}
	if len(pods.Items) == 0 {
		return nil, errors.Wrapf(errNoKSMPodsFound, "no KSM pod found with label: '%s'", p.ksmPodLabel)
	}

	var foundPods []v1.Pod
	for _, pod := range pods.Items {
		if pod.Status.HostIP == "" {
			continue
		}

		if pod.Status.HostIP == p.ownNodeIP {
			p.logger.Debugf("Found KSM pod running on this code, pod IP: %s", pod.Status.PodIP)
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
		ksmClient := &ksm{
			nodeIP:   pod.Status.HostIP,
			endpoint: endpoint,
			httpClient: &http.Client{
				Timeout: timeout,
			},
			logger: p.logger,
		}
		clients = append(clients, ksmClient)
	}
	return clients, nil
}

// NewPodLabelDiscoverer creates a new KSM discoverer that will find KSM pods using k8s labels
func NewDistributedPodLabelDiscoverer(ksmPodLabel string, nodeIP string, logger *logrus.Logger, k8sClient client.Kubernetes) client.MultiDiscoverer {
	return &distributedPodLabelDiscoverer{
		logger:      logger,
		ownNodeIP:   nodeIP,
		k8sClient:   k8sClient,
		ksmPodLabel: ksmPodLabel,
	}
}
