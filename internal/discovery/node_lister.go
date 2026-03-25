package discovery

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
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

// restNodeLister implements listersv1.NodeLister using direct REST calls to the
// nodes/status subresource. This requires only nodes/status RBAC (get) rather
// than nodes (list/watch), which avoids the implicit nodes/proxy grant.
type restNodeLister struct {
	rc rest.Interface
}

// NewRestNodeLister returns a NodeLister backed by direct REST calls to nodes/status.
// Use this instead of NewNodeLister when kubeletFineGrainedAuth is enabled, so that
// the SA only needs nodes/status permission and not the broader nodes resource.
func NewRestNodeLister(inClusterConfig *rest.Config) (listersv1.NodeLister, error) {
	cfg := *inClusterConfig
	cfg.APIPath = "/api"
	cfg.GroupVersion = &corev1.SchemeGroupVersion
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	rc, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return nil, fmt.Errorf("building REST client for node status lister: %w", err)
	}

	return &restNodeLister{rc: rc}, nil
}

func (r *restNodeLister) Get(name string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := r.rc.Get().
		Resource("nodes").
		Name(name).
		SubResource("status").
		Do(context.Background()).
		Into(node)
	if err != nil {
		return nil, fmt.Errorf("getting node %q status: %w", name, err)
	}
	return node, nil
}

// List is not needed by the kubelet scraper (only Get is used), but must be
// implemented to satisfy the NodeLister interface.
func (r *restNodeLister) List(_ labels.Selector) ([]*corev1.Node, error) {
	return nil, fmt.Errorf("List not supported by restNodeLister; use Get")
}
