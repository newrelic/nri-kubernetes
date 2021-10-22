package ksm

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
)

const defaultLabelSelector = "app.kubernetes.io/name=kube-state-metrics"

type EndpointsLister interface {
	List(selector labels.Selector) (ret []*corev1.Endpoints, err error)
}

type DiscoveryConfig struct {
	LabelSelector   string
	Namespace       string
	EndpointsLister func(...informers.SharedInformerOption) EndpointsLister
}

type Discoverer interface {
	Discover() ([]string, error)
}

type discoverer struct {
	endpointsLister EndpointsLister
}

func NewDiscoverer(config DiscoveryConfig) (Discoverer, error) {
	labelSelector := config.LabelSelector
	if labelSelector == "" {
		labelSelector = defaultLabelSelector
	}

	if config.EndpointsLister == nil {
		return nil, fmt.Errorf("endpoints lister factory must be configured")
	}

	return &discoverer{
		endpointsLister: config.EndpointsLister(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *v1.ListOptions) {
				options.LabelSelector = labelSelector
			}),
		),
	}, nil
}

func (d *discoverer) Discover() ([]string, error) {
	endpoints, err := d.endpointsLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints discovered")
	}

	hosts := []string{}

	for _, endpoint := range endpoints {
		for _, subset := range endpoint.Subsets {
			for _, address := range subset.Addresses {
				for _, port := range subset.Ports {
					hosts = append(hosts, net.JoinHostPort(address.IP, strconv.Itoa(int(port.Port))))
				}
			}
		}
	}

	return hosts, nil
}
