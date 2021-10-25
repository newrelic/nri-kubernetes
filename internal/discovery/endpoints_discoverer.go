package discovery

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
)

type EndpointsDiscoveryConfig struct {
	LabelSelector   string
	Namespace       string
	EndpointsLister func(...informers.SharedInformerOption) EndpointsLister
}

type EndpointsLister interface {
	List(selector labels.Selector) (ret []*corev1.Endpoints, err error)
}

type EndpointsDiscoverer interface {
	Discover() ([]string, error)
}

type endpointsDiscoverer struct {
	endpointsLister EndpointsLister
}

func NewEndpointsDiscoverer(config EndpointsDiscoveryConfig) (EndpointsDiscoverer, error) {
	if config.EndpointsLister == nil {
		return nil, fmt.Errorf("endpoints lister factory must be configured")
	}

	return &endpointsDiscoverer{
		endpointsLister: config.EndpointsLister(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *v1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
	}, nil
}

func (d *endpointsDiscoverer) Discover() ([]string, error) {
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
