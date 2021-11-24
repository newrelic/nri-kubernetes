package discovery

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// NodeGetter get nodes with informers.
type NodeGetter interface {
	Get(name string) (ret *corev1.Node, err error)
}

// NewNodesGetter returns a NodeGetter to get nodes with informers.
func NewNodesGetter(client kubernetes.Interface, options ...informers.SharedInformerOption) (NodeGetter, chan<- struct{}) {
	stopCh := make(chan struct{})

	factory := informers.NewSharedInformerFactoryWithOptions(client, defaultResyncDuration, options...)

	nodeGetter := factory.Core().V1().Nodes().Lister()

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return nodeGetter, stopCh
}

// MockedNodeGetter is a simple lister that returns an hardcoded node.
// For integration testing, it is recommended to use a MockedNodeGetter with testutil.FakeK8sClient as a backend.
type MockedNodeGetter struct {
	Node *corev1.Node
}

func (m MockedNodeGetter) Get(name string) (ret *corev1.Node, err error) {
	return m.Node, nil
}
