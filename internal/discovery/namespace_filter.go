package discovery

import (
	"errors"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

const defaultNamespaceResyncDuration = 10 * time.Minute

// NamespaceFilterer provides an interface to filter from a given namespace.
type NamespaceFilterer interface {
	IsAllowed(namespace string) bool
}

// NamespaceFilter is a struct holding pointers to the config and the namespace lister.
type NamespaceFilter struct {
	c      *config.NamespaceSelector
	lister listersv1.NamespaceLister
	stopCh chan<- struct{}
	logger *log.Logger
}

// NewNamespaceFilter inits the namespace lister and returns a new NamespaceFilter.
func NewNamespaceFilter(c *config.NamespaceSelector, client kubernetes.Interface, logger *log.Logger, options ...informers.SharedInformerOption) *NamespaceFilter {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultNamespaceResyncDuration, options...)

	lister := factory.Core().V1().Namespaces().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return &NamespaceFilter{
		c:      c,
		lister: lister,
		stopCh: stopCh,
		logger: logger,
	}
}

// IsAllowed checks if a namespace is allowed to be scraped given a certain match labels or expressions configuration.
func (nf *NamespaceFilter) IsAllowed(namespace string) bool {
	if nf.c == nil {
		log.Tracef("Allowing %q namespace as selector is nil", namespace)
		return true
	}

	if nf.c.MatchLabels != nil {
		log.Tracef("Filtering %q namespace by MatchLabels", namespace)
		return nf.matchNamespaceByLabels(namespace)
	}

	if nf.c.MatchExpressions != nil {
		log.Tracef("Filtering %q namespace by MatchExpressions", namespace)
		return nf.matchNamespaceByExpressions(namespace)
	}

	return true
}

// matchNamespaceByLabels filters a namespace using the selector from the MatchLabels config.
func (nf *NamespaceFilter) matchNamespaceByLabels(namespace string) bool {
	namespaceList, err := nf.lister.List(labels.SelectorFromSet(nf.c.MatchLabels))
	if err != nil {
		nf.logger.Errorf("listing namespaces with MatchLabels: %v", err)
		return true
	}

	return containsNamespace(namespace, namespaceList)
}

// matchNamespaceByExpressions filters a namespace using the selector from the MatchExpressions config.
func (nf *NamespaceFilter) matchNamespaceByExpressions(namespace string) bool {
	for _, expression := range nf.c.MatchExpressions {
		selector, err := labels.Parse(expression.String())
		if err != nil {
			nf.logger.Errorf("parsing labels: %v", err)
			return true
		}

		namespaceList, err := nf.lister.List(selector)
		if err != nil {
			nf.logger.Errorf("listing namespaces with MatchExpressions: %v", err)
			return true
		}

		if !containsNamespace(namespace, namespaceList) {
			return false
		}
	}

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

// CachedNamespaceFilter is a wrapper of the NamespaceFilterer and the cache.
type CachedNamespaceFilter struct {
	NsFilter NamespaceFilterer
	cache    storer.Storer
}

// NewCachedNamespaceFilter create a new CachedNamespaceFilter, wrapping the cache and the NamespaceFilterer.
func NewCachedNamespaceFilter(ns NamespaceFilterer, storer storer.Storer) *CachedNamespaceFilter {
	return &CachedNamespaceFilter{
		NsFilter: ns,
		cache:    storer,
	}
}

// IsAllowed check the cache and calls the underlying NamespaceFilter if the result is not found.
func (cnf *CachedNamespaceFilter) IsAllowed(namespace string) bool {
	// Check if the namespace is already in the cache.
	var allowed bool
	if _, err := cnf.cache.Get(namespace, &allowed); err == nil {
		return allowed
	}

	allowed = cnf.NsFilter.IsAllowed(namespace)

	// Save the namespace in the cache.
	_ = cnf.cache.Set(namespace, allowed)

	return allowed
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
