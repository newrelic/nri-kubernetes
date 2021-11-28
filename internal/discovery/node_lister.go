package discovery

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
)

// NewNodeLister returns a NodeGetter to get nodes with informers.
func NewNodeLister(client kubernetes.Interface, options ...informers.SharedInformerOption) (listersv1.NodeLister, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	nodeGetter := factory.Core().V1().Nodes().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return nodeGetter, stopCh
}
