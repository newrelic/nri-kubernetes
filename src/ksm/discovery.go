package ksm

import "github.com/newrelic/nri-kubernetes/v2/internal/discovery"

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"

type DiscoveryConfig struct {
	discovery.EndpointsDiscoveryConfig
}

func NewDiscoverer(config DiscoveryConfig) (discovery.EndpointsDiscoverer, error) {
	if config.LabelSelector == "" {
		config.LabelSelector = defaultLabelSelector
	}

	return discovery.NewEndpointsDiscoverer(config.EndpointsDiscoveryConfig)
}
