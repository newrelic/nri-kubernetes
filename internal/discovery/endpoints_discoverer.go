package discovery

import (
	"fmt"
	"net"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
)

type EndpointsDiscoveryConfig struct {
	LabelSelector string
	Namespace     string
	Port          int
	FixedEndpoint []string

	EndpointsLister func(...informers.SharedInformerOption) EndpointsLister
}

type EndpointsLister interface {
	List(selector labels.Selector) (ret []*corev1.Endpoints, err error)
}

type EndpointsDiscoverer interface {
	Discover() ([]string, error)
}

type endpointsDiscoverer struct {
	endpointsLister     EndpointsLister
	port                int
	fixedEndpointSorted []string
}

func NewEndpointsDiscoverer(config EndpointsDiscoveryConfig) (EndpointsDiscoverer, error) {
	if config.EndpointsLister == nil && config.FixedEndpoint == nil {
		return nil, fmt.Errorf("endpoints lister factory must be configured")
	}

	if config.FixedEndpoint != nil {
		// Sorting the array is needed to be sure we are hitting each time the endpoints in the same order
		sort.Strings(config.FixedEndpoint)
	}

	return &endpointsDiscoverer{
		endpointsLister: config.EndpointsLister(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *v1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
		port:                config.Port,
		fixedEndpointSorted: config.FixedEndpoint,
	}, nil
}

func (d *endpointsDiscoverer) Discover() ([]string, error) {
	if len(d.fixedEndpointSorted) != 0 {
		return d.fixedEndpointSorted, nil
	}

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
					if d.port != 0 && d.port != int(port.Port) {
						continue
					}

					hosts = append(hosts, net.JoinHostPort(address.IP, strconv.Itoa(int(port.Port))))
				}
			}
		}
	}

	// Sorting the array is needed to be sure we are hitting each time the endpoints in the same order
	sort.Strings(hosts)

	return hosts, nil
}
