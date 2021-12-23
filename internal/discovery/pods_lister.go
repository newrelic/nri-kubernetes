package discovery

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// PodListerer return namespaced pod listers.
type PodListerer interface {
	// Lister ruturns the pod lister for the specified namespaces
	// and true if the lister exist in the listerer.
	Lister(namespace string) (listersv1.PodNamespaceLister, bool)
}

type PodListererConfig struct {
	// Namespaces supported by the listerer.
	Namespaces []string
	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

// MultiNamespacePodListerer implements PodListerer interface
// for a group of listers pre-build on initialization.
type MultiNamespacePodListerer struct {
	listers map[string]listersv1.PodNamespaceLister
}

// Lister returns the available lister based on the namespace if exists in the listerer.
func (l MultiNamespacePodListerer) Lister(namespace string) (listersv1.PodNamespaceLister, bool) {
	lister, ok := l.listers[namespace]

	return lister, ok
}

// NewNamespacePodListerer returns a MultiNamespacePodListerer with listers for all
// namespaces on config.Namespaces.
func NewNamespacePodListerer(config PodListererConfig) (*MultiNamespacePodListerer, chan<- struct{}) {
	stopCh := make(chan struct{})

	multiNamespacePodListerer := &MultiNamespacePodListerer{
		listers: make(map[string]listersv1.PodNamespaceLister),
	}

	for _, namespace := range config.Namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(
			config.Client,
			defaultResyncDuration,
			informers.WithNamespace(namespace),
		)

		multiNamespacePodListerer.listers[namespace] = factory.Core().V1().Pods().Lister().Pods(namespace)

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)
	}

	return multiNamespacePodListerer, stopCh
}
