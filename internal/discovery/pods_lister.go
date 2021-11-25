package discovery

import (
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
)

type PodsListerConfig struct {
	// Namespace can be used to restric the search to a particular namespace.
	Namespace string
	// Client is the Kubernetes client.Interface used to build informers.
	Client kubernetes.Interface
}

func NewPodsLister(config PodsListerConfig) (v1.PodLister, chan<- struct{}) {
	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute
	factory := informers.NewSharedInformerFactoryWithOptions(
		config.Client,
		resyncDuration,
		informers.WithNamespace(config.Namespace),
	)

	podsLister := factory.Core().V1().Pods().Lister()

	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return podsLister, stopCh
}
