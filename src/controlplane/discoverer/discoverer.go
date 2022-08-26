package discoverer

import (
	"errors"
	"fmt"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

var ErrPodNotFound = errors.New("pod not found")

// PodDiscoverer is used to discover control plane components.
type PodDiscoverer interface {
	// Discover returns a pod matching the selector, namespaces and
	// is in the same node if matchNode is true.
	Discover(config.AutodiscoverControlPlane) (*corev1.Pod, error)
}

type Config struct {
	PodListerer discovery.PodListerer
	NodeName    string
}

type OptionFunc func(c *ControlplanePodDiscoverer) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(c *ControlplanePodDiscoverer) error {
		c.logger = logger
		return nil
	}
}

// ControlplanePodDiscoverer implements PodDiscoverer interface.
type ControlplanePodDiscoverer struct {
	Config
	logger *log.Logger
}

// New returns an ControlplanePodDiscoverer.
func New(config Config, opts ...OptionFunc) (*ControlplanePodDiscoverer, error) {
	c := &ControlplanePodDiscoverer{
		logger: logutil.Discard,
		Config: config,
	}

	for i, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return c, nil
}

// Discover returns the first Pod matching the namespace and selector from the listed pods.
// If MatchNode is true the Pod must be running on the same node to match.
//
// Errors returned by this function should be managed as severe and not related to the
// autodiscover entry. No error is returned if no Pod has been discovered.
func (c *ControlplanePodDiscoverer) Discover(ad config.AutodiscoverControlPlane) (*corev1.Pod, error) {
	podLister, ok := c.PodListerer.Lister(ad.Namespace)
	if !ok {
		return nil, fmt.Errorf("pod lister for namespace: %s not found", ad.Namespace)
	}

	labelsSet, err := labels.ConvertSelectorToLabelsMap(ad.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector %q: %w", ad.Selector, err)
	}

	selector := labels.SelectorFromSet(labelsSet)

	pods, err := podLister.List(selector)
	if err != nil {
		return nil, fmt.Errorf("listing pods with selector %q: %w", labelsSet, err)
	}

	c.logger.Debugf("%d pods found with labels %q", len(pods), ad.Selector)

	for _, pod := range pods {
		if ad.MatchNode && pod.Spec.NodeName != c.Config.NodeName {
			c.logger.Debugf("Discarding pod: %s running outside the node", pod.Name)
			continue
		}
		// first pod matching all conditions is returned.
		return pod, nil
	}

	return nil, ErrPodNotFound
}
