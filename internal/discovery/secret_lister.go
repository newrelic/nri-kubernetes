package discovery

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

type SecretListerer interface {
	Lister(namespace string) (listersv1.SecretNamespaceLister, bool)
}

type SecretListerConfig struct {
	// Namespace can be used to restric the search to a particular namespace.
	Namespaces []string
	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

type MultiNamespaceSecretListerer struct {
	listers map[string]listersv1.SecretNamespaceLister
}

func (l MultiNamespaceSecretListerer) Lister(namespace string) (listersv1.SecretNamespaceLister, bool) {
	lister, ok := l.listers[namespace]

	return lister, ok
}

// NewSecretNamespaceLister returns a SecretGetter to get secrets with informers.
func NewSecretNamespaceLister(config SecretListerConfig) (*MultiNamespaceSecretListerer, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(
		config.Client,
		defaultResyncDuration,
	)

	multiNamespaceSecretListerer := &MultiNamespaceSecretListerer{
		listers: make(map[string]listersv1.SecretNamespaceLister),
	}

	for _, namespace := range config.Namespaces {
		multiNamespaceSecretListerer.listers[namespace] = factory.Core().V1().Secrets().Lister().Secrets(namespace)
	}

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return multiNamespaceSecretListerer, stopCh
}
