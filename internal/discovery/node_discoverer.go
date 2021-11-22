package discovery

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type NodeGetter interface {
	Get(name string) (ret *corev1.Node, err error)
}

func NewNodesGetter(client kubernetes.Interface, options ...informers.SharedInformerOption) (NodeGetter, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	lister := factory.Core().V1().Nodes().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return lister, stopCh
}

// MockedServicesLister is a simple lister that returns an hardcoded node.
// For integration testing, it is recommended to use the a MockedNodeGetter with testutil.FakeK8sClient as a backend.
type MockedNodeGetter struct {
	Node *corev1.Node
}

func (m MockedNodeGetter) Get(name string) (ret *corev1.Node, err error) {
	return m.Node, nil
}

type NodeDiscoverer interface {
	Discover(name string) (*corev1.Node, error)
}

type nodesDiscoverer struct {
	NodeGetter NodeGetter
}

func (d *nodesDiscoverer) Discover(nodeName string) (*corev1.Node, error) {

	node, err := d.NodeGetter.Get(nodeName)
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	return node, nil
}

func NewNodeDiscoverer(client kubernetes.Interface) NodeDiscoverer {

	nl, _ := NewNodesGetter(client)

	return &nodesDiscoverer{
		NodeGetter: nl,
	}
}
