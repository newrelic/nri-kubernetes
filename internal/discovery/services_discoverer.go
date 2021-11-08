package discovery

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type ServicesLister interface {
	List(selector labels.Selector) (ret []*corev1.Service, err error)
}

type ServiceDiscoverer interface {
	Discover() ([]*corev1.Service, error)
}

type servicesDiscoverer struct {
	ServicesLister ServicesLister
}

func (d *servicesDiscoverer) Discover() ([]*corev1.Service, error) {

	services, err := d.ServicesLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	return services, nil
}

func NewServicesDiscoverer(client kubernetes.Interface) ServiceDiscoverer {
	// Arbitrary value, same used in Prometheus.
	resyncDuration := 10 * time.Minute
	stopCh := make(chan struct{})
	sl := func(options ...informers.SharedInformerOption) ServicesLister {
		factory := informers.NewSharedInformerFactoryWithOptions(client, resyncDuration, options...)

		lister := factory.Core().V1().Services().Lister()

		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)

		return lister
	}

	return &servicesDiscoverer{
		ServicesLister: sl(),
	}
}
