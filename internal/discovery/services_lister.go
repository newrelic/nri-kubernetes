package discovery

import (
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// defaultResyncDuration is an arbitrary value, same used in Prometheus.
const defaultResyncDuration = 10 * time.Minute

func NewServicesLister(client kubernetes.Interface, options ...informers.SharedInformerOption) (listersv1.ServiceLister, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	lister := factory.Core().V1().Services().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return lister, stopCh
}
