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

// EndpointsDiscoveryConfig configures endpoint discovery for services.
// Used by both EndpointSlice and legacy Endpoints discoverers.
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

// ErrDiscoveryTimeout is returned by EndpointsDiscovererWithTimeout when discovery times out.
var ErrDiscoveryTimeout = errors.New("timeout discovering endpoints")

// ErrClientNotConfigured is returned when the Kubernetes client is not provided.
var ErrClientNotConfigured = errors.New("client must be configured")

// EndpointsDiscovererWithTimeout wraps an EndpointsDiscoverer with retry/timeout logic.
// It polls the inner discoverer until endpoints are found or timeout is reached.
type EndpointsDiscovererWithTimeout struct {
	EndpointsDiscoverer
	BackoffDelay time.Duration
	Timeout      time.Duration
}

// Discover polls the inner EndpointsDiscoverer every BackoffDelay until it returns:
// - An error (fail immediately).
// - A non-empty list of endpoints (success).
// - Timeout expires (return ErrDiscoveryTimeout).
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
	lister              discoverylisters.EndpointSliceLister
	port                int
	fixedEndpointSorted []string
}

// NewEndpointSliceDiscoverer creates a new EndpointsDiscoverer that uses the EndpointSlice API.
// This is the modern replacement for the deprecated v1 Endpoints API.
//
//nolint:ireturn // Returning interface is correct design for abstraction.
func NewEndpointSliceDiscoverer(config EndpointsDiscoveryConfig) (EndpointsDiscoverer, error) {
	if config.Client == nil {
		return nil, ErrClientNotConfigured
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

	return &endpointSliceDiscoverer{
		lister: esl(
			informers.WithNamespace(config.Namespace),
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = config.LabelSelector
			}),
		),
		port: config.Port,
	}, nil
}

// Discover returns a list of "host:port" strings for all ready endpoints in matching EndpointSlices.
// The output is sorted alphabetically and deduplicated across multiple slices.
//
//nolint:gocognit,gocyclo,cyclop // Nested loops match EndpointSlice structure (slices->endpoints->addresses->ports).
func (d *endpointSliceDiscoverer) Discover() ([]string, error) {
	// If fixed endpoints are set, return them (same as legacy discoverer)
	if len(d.fixedEndpointSorted) != 0 {
		return d.fixedEndpointSorted, nil
	}

	slices, err := d.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing endpointslices: %w", err)
	}

	var hosts []string
	seen := make(map[string]struct{}) // Deduplicate across multiple slices

	for _, slice := range slices {
		// Process each EndpointSlice
		for _, endpoint := range slice.Endpoints {
			// Skip if endpoint is not ready
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

					// Deduplicate: only add if not already seen
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
