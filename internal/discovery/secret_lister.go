package discovery

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// SecretListerer return namespaced secret listers.
type SecretListerer interface {
	// Lister ruturns the secret lister for the specified namespaces
	// and true if the lister exist in the listerer.
	Lister(namespace string) (listersv1.SecretNamespaceLister, bool)
}

type SecretListererConfig struct {
	// Namespaces supported by the listerer.
	Namespaces []string
	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

// MultiNamespaceSecretListerer implements SecretListerer interface
// for a group of listers pre-build on initialization.
type MultiNamespaceSecretListerer struct {
	listers map[string]listersv1.SecretNamespaceLister
}

// Lister returns the available lister based on the namespace if exists in the listerer.
func (l MultiNamespaceSecretListerer) Lister(namespace string) (listersv1.SecretNamespaceLister, bool) {
	lister, ok := l.listers[namespace]

	return lister, ok
}

// NewNamespaceSecretListerer returns a MultiNamespaceSecretListerer with listers for all
// namespaces on config.Namespaces.
func NewNamespaceSecretListerer(config SecretListererConfig) (*MultiNamespaceSecretListerer, chan<- struct{}) {
	stopCh := make(chan struct{})

	multiNamespaceSecretListerer := &MultiNamespaceSecretListerer{
		listers: make(map[string]listersv1.SecretNamespaceLister),
	}

	for _, namespace := range config.Namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(
			config.Client,
			defaultResyncDuration,
			informers.WithNamespace(namespace),
		)

		multiNamespaceSecretListerer.listers[namespace] = factory.Core().V1().Secrets().Lister().Secrets(namespace)

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)
	}

	return multiNamespaceSecretListerer, stopCh
}
