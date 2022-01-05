package discovery

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

type EndpointsDiscoveryConfig struct {
	// LabelSelector is the selector used to filter Endpoints.
	LabelSelector string
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string
	// If set, Port will discard all endpoints discovered that do not use this specified port
	Port int

	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

type EndpointsDiscoverer interface {
	Discover() ([]string, error)
}

type endpointsDiscoverer struct {
	lister              listersv1.EndpointsLister
	port                int
	fixedEndpointSorted []string
}

func NewEndpointsDiscoverer(config EndpointsDiscoveryConfig) (EndpointsDiscoverer, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("client must be configured")
	}

	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute
	stopCh := make(chan struct{})
	el := func(options ...informers.SharedInformerOption) listersv1.EndpointsLister {
		factory := informers.NewSharedInformerFactoryWithOptions(config.Client, resyncDuration, options...)

		lister := factory.Core().V1().Endpoints().Lister()

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)

		return lister
	}

	return &endpointsDiscoverer{
		lister: el(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
		port: config.Port,
	}, nil
}

func (d *endpointsDiscoverer) Discover() ([]string, error) {
	if len(d.fixedEndpointSorted) != 0 {
		return d.fixedEndpointSorted, nil
	}

	endpoints, err := d.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing endpoints: %w", err)
	}

	var hosts []string

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

// ErrDiscoveryTimeout is returned by EndpointsDiscovererWithTimeout when discovery times out
var ErrDiscoveryTimeout = errors.New("timeout discovering endpoints")

// EndpointsDiscovererWithTimeout implements EndpointsDiscoverer with a retry mechanism if no endpoints are found.
type EndpointsDiscovererWithTimeout struct {
	EndpointsDiscoverer
	BackoffDelay time.Duration
	Timeout      time.Duration
}

// Discover will call poll the inner EndpointsDiscoverer every BackoffDelay seconds up to a max of Retries times until it
// returns an error, or a non-empty list of endpoints.
// If the max number of Retries is exceeded, it will return ErrDiscoveryTimeout.
func (edt *EndpointsDiscovererWithTimeout) Discover() ([]string, error) {
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

	return nil, ErrDiscoveryTimeout
}
