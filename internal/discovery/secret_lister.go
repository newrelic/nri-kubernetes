package discovery

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

type SecretListerConfig struct {
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string
	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

// NewSecretNamespaceLister returns a SecretGetter to get secrets with informers.
func NewSecretNamespaceLister(config SecretListerConfig) (listersv1.SecretNamespaceLister, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(
		config.Client,
		defaultResyncDuration,
		informers.WithNamespace(config.Namespace),
	)

	secretLister := factory.Core().V1().Secrets().Lister().Secrets(config.Namespace)

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return secretLister, stopCh
}
