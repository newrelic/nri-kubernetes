package discovery

import (
	"errors"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

const (
	defaultNamespaceResyncDuration = 10 * time.Minute
)

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
	namespaceList, err := nf.lister.List(labels.SelectorFromSet(nf.parseToStringMap(nf.c.MatchLabels)))
	if err != nil {
		nf.logger.Errorf("listing namespaces with MatchLabels: %v", err)
		return true
	}

	return containsNamespace(namespace, namespaceList)
}

// matchNamespaceByExpressions filters a namespace using the selector from the MatchExpressions config.
func (nf *NamespaceFilter) matchNamespaceByExpressions(namespace string) bool {
	for _, expression := range nf.c.MatchExpressions {
		val, err := expression.String()
		if err != nil {
			nf.logger.Error(err)
			return true
		}

		selector, err := labels.Parse(val)
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

func (nf *NamespaceFilter) parseToStringMap(matchLabels map[string]interface{}) map[string]string {
	strMap := make(map[string]string)

	for k, v := range matchLabels {
		val, ok := v.(string)
		if !ok {
			nf.logger.Errorf("parseToStringMap value into string: %v, type: %t", v, v)
			continue
		}
		strMap[k] = val
	}

	return strMap
}

// Close closes the stop channel and implements the Closer interface.
func (nf *NamespaceFilter) Close() error {
	if nf.stopCh == nil {
		return errors.New("invalid channel")
	}

	close(nf.stopCh)

	return nil
}

// CachedNamespaceFilter holds a NamespaceCache around the NamespaceFilterer.
type CachedNamespaceFilter struct {
	cache  NamespaceCache
	filter NamespaceFilterer
}

// NewCachedNamespaceFilter create a new CachedNamespaceFilter, wrapping the cache and the NamespaceFilterer.
func NewCachedNamespaceFilter(filter NamespaceFilterer, cache NamespaceCache) *CachedNamespaceFilter {
	return &CachedNamespaceFilter{
		filter: filter,
		cache:  cache,
	}
}

// IsAllowed looks for a match in the cache first, otherwise calls the filter.
func (cm *CachedNamespaceFilter) IsAllowed(namespace string) bool {
	if match, found := cm.cache.Match(namespace); found {
		return match
	}

	match := cm.filter.IsAllowed(namespace)
	cm.cache.Put(namespace, match)

	return match
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
