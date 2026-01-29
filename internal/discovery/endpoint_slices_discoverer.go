package discovery

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	apidiscoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	discoverylistersv1 "k8s.io/client-go/listers/discovery/v1"
)

type EndpointSlicesDiscoveryConfig struct {
	// LabelSelector is the selector used to filter Endpoints.
	LabelSelector string
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string
	// If set, Port will discard all endpoints discovered that do not use this specified port
	Port int

	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

type EndpointSlicesDiscoverer interface {
	Discover() ([]string, error)
}

type endpointSlicesDiscoverer struct {
	lister              discoverylistersv1.EndpointSliceLister
	port                int
	fixedEndpointSorted []string
}

func NewEndpointSlicesDiscoverer(config EndpointSlicesDiscoveryConfig) (EndpointSlicesDiscoverer, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("client must be configured")
	}

	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute
	stopCh := make(chan struct{})

	var _ = apidiscoveryv1.EndpointSlice{}

	el := func(options ...informers.SharedInformerOption) discoverylistersv1.EndpointSliceLister {
		factory := informers.NewSharedInformerFactoryWithOptions(config.Client, resyncDuration, options...)

		lister := factory.Discovery().V1().EndpointSlices().Lister()

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)

		return lister
	}

	return &endpointSlicesDiscoverer{
		lister: el(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
		port: config.Port,
	}, nil
}

func (d *endpointSlicesDiscoverer) Discover() ([]string, error) {
	if len(d.fixedEndpointSorted) != 0 {
		return d.fixedEndpointSorted, nil
	}

	endpointSlices, err := d.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing endpoints: %w", err)
	}

	var hosts []string

	for _, endpointSlice := range endpointSlices {
		for _, endpoint := range endpointSlice.Endpoints {
			for _, address := range endpoint.Addresses {
				for _, port := range endpointSlice.Ports {
					if port.Port == nil {
						continue
					}
					if d.port != 0 && d.port != int(*port.Port) {
						continue
					}

					//@todo: validate if these checks are needed
					if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
						continue
					}
					if endpoint.Conditions.Serving != nil && !*endpoint.Conditions.Serving {
						continue
					}
					if endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating {
						continue
					}

					hosts = append(hosts, net.JoinHostPort(address, strconv.Itoa(int(*port.Port))))
				}
			}
		}
	}

	// Sorting the array is needed to be sure we are hitting each time the endpoints in the same order
	sort.Strings(hosts)

	return hosts, nil
}

// ErrEnpointSlicesDiscoveryTimeout is returned by EndpointsDiscovererWithTimeout when discovery times out
var ErrEnpointSlicesDiscoveryTimeout = errors.New("timeout discovering endpoint slices")

// EndpointsDiscovererWithTimeout implements EndpointsDiscoverer with a retry mechanism if no endpoints are found.
type EndpointSlicesDiscovererWithTimeout struct {
	EndpointsDiscoverer
	BackoffDelay time.Duration
	Timeout      time.Duration
}

// Discover will call poll the inner EndpointsDiscoverer every BackoffDelay seconds up to a max of Retries times until it
// returns an error, or a non-empty list of endpoints.
// If the max number of Retries is exceeded, it will return ErrDiscoveryTimeout.
func (edt *EndpointSlicesDiscovererWithTimeout) Discover() ([]string, error) {
	start := time.Now()
	for time.Since(start) < edt.Timeout {
		endpoints, err := edt.EndpointsDiscoverer.Discover()
		if err != nil {
			return nil, err
		}

		if len(endpoints) > 0 {
			return endpoints, nil
		}

		time.Sleep(edt.BackoffDelay)
	}

	return nil, ErrEnpointSlicesDiscoveryTimeout
}
