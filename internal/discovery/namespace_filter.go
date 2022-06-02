package discovery

import (
	"errors"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// NamespaceFilterer provides an interface to filter namespaces.
type NamespaceFilterer interface {
	IsAllowed(namespace string) bool
}

// NamespaceFilter is a struct holding pointers to the config and the namespace lister.
type NamespaceFilter struct {
	c      *config.Config
	lister listersv1.NamespaceLister
	stopCh chan<- struct{}
	storer storer.Storer
}

// NewNamespaceFilter inits the namespace lister and returns a new NamespaceFilter.
func NewNamespaceFilter(c *config.Config, client kubernetes.Interface, storer storer.Storer, options ...informers.SharedInformerOption) *NamespaceFilter {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	lister := factory.Core().V1().Namespaces().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return &NamespaceFilter{
		c:      c,
		lister: lister,
		stopCh: stopCh,
		storer: storer,
	}
}

// IsAllowed checks given any namespace, if it's allowed to be scraped by using the NamespaceLister
func (nf *NamespaceFilter) IsAllowed(namespace string) bool {
	// By default, we scrape every namespace.
	if nf.c.NamespaceSelector == nil {
		return true
	}

	// Check if the namespace is already in the cache.
	var allowed bool
	if _, err := nf.storer.Get(namespace, &allowed); err == nil {
		return allowed
	}

	// Scrape namespaces by honoring the matchLabels values.
	if nf.c.NamespaceSelector.MatchLabels != nil {
		namespaceList, err := nf.lister.List(labels.SelectorFromSet(nf.c.NamespaceSelector.MatchLabels))
		if err != nil {
			log.Errorf("listing namespaces with MatchLabels: %v", err)
			return true
		}

		allowed = containsNamespace(namespace, namespaceList)

		// Save the namespace in the cache.
		_ = nf.storer.Set(namespace, allowed)

		return allowed
	}

	// Scrape namespaces by honoring the matchExpressions values.
	// Multiple expressions are evaluated with a logical AND between them.
	if nf.c.NamespaceSelector.MatchExpressions != nil {
		for _, expression := range nf.c.NamespaceSelector.MatchExpressions {
			selector, err := labels.Parse(expression.String())
			if err != nil {
				log.Errorf("parsing labels: %v", err)
				return true
			}

			namespaceList, err := nf.lister.List(selector)
			if err != nil {
				log.Errorf("listing namespaces with MatchExpressions: %v", err)
				return true
			}

			if !containsNamespace(namespace, namespaceList) {
				return false
			}
		}
	}

	// Save the namespace in the cache.
	_ = nf.storer.Set(namespace, true)

	return true
}

// Close closes the stop channel and implements the Closer interface.
func (nf *NamespaceFilter) Close() error {
	if nf.stopCh == nil {
		return errors.New("invalid channel")
	}

	close(nf.stopCh)

	return nil
}

// containsNamespace checks if a namespaces is contained in a given list of namespaces.
func containsNamespace(namespace string, namespaceList []*v1.Namespace) bool {
	for _, n := range namespaceList {
		if n.Name == namespace {
			return true
		}
	}

	return false
}
