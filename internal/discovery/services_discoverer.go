package discovery

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// defaultResyncDuration is an arbitrary value, same used in Prometheus.
const defaultResyncDuration = 10 * time.Minute

type ServicesLister interface {
	List(selector labels.Selector) (ret []*corev1.Service, err error)
}

func NewServicesLister(client kubernetes.Interface, options ...informers.SharedInformerOption) (ServicesLister, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	lister := factory.Core().V1().Services().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return lister, stopCh
}

// MockedServicesLister is a simple lister that returns a hardcoded list of services.
// For integration testing, it is recommended to use the a ServiceDiscoverer with testutil.FakeK8sClient as a backend.
type MockedServicesLister struct {
	Services []*corev1.Service
}

func (msl MockedServicesLister) List(selector labels.Selector) (ret []*corev1.Service, err error) {
	return msl.Services, nil
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

	sl, _ := NewServicesLister(client)

	return &servicesDiscoverer{
		ServicesLister: sl,
	}
}
