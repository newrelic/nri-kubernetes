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
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
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

var ErrDiscoveryTimeout = errors.New("timeout discovering endpoints")

var ErrClientNotConfigured = errors.New("client must be configured")

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
			return nil, fmt.Errorf("discovering endpoints: %w", err)
		}

		if len(endpoints) > 0 {
			return endpoints, nil
		}

		time.Sleep(edt.BackoffDelay)
	}

	return nil, ErrDiscoveryTimeout
}

type endpointSliceDiscoverer struct {
	lister discoverylisters.EndpointSliceLister
	port   int
}

//nolint:ireturn // Returning interface is correct design for abstraction.
func NewEndpointSliceDiscoverer(config EndpointsDiscoveryConfig) (EndpointsDiscoverer, chan<- struct{}, error) {
	if config.Client == nil {
		return nil, nil, ErrClientNotConfigured
	}

	// Arbitrary value, same used in Prometheus and legacy Endpoints discoverer
	resyncDuration := 10 * time.Minute
	stopCh := make(chan struct{})

	esl := func(options ...informers.SharedInformerOption) discoverylisters.EndpointSliceLister {
		factory := informers.NewSharedInformerFactoryWithOptions(config.Client, resyncDuration, options...)

		lister := factory.Discovery().V1().EndpointSlices().Lister()

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)

		return lister
	}

	discoverer := &endpointSliceDiscoverer{
		lister: esl(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
		port: config.Port,
	}

	return discoverer, stopCh, nil
}

//nolint:gocognit,gocyclo,cyclop // Nested loops match EndpointSlice structure (slices->endpoints->addresses->ports).
func (d *endpointSliceDiscoverer) Discover() ([]string, error) {
	slices, err := d.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing endpointslices: %w", err)
	}

	var hosts []string
	seen := make(map[string]struct{}) // Deduplicate across multiple slices

	for _, slice := range slices {
		for _, endpoint := range slice.Endpoints {
			// Note: If Ready is nil, we treat it as ready for backward compatibility
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}

			for _, address := range endpoint.Addresses {
				for _, port := range slice.Ports {
					if port.Port == nil {
						continue
					}

					if d.port != 0 && d.port != int(*port.Port) {
						continue
					}

					host := net.JoinHostPort(address, strconv.Itoa(int(*port.Port)))

					if _, exists := seen[host]; !exists {
						hosts = append(hosts, host)
						seen[host] = struct{}{}
					}
				}
			}
		}
	}

	sort.Strings(hosts)

	return hosts, nil
}
