package apiserver

import (
	"context"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Client an interface for querying the k8s API server
type Client interface {
	GetNodeInfo(nodeName string) (*NodeInfo, error)
}

// NewClient creates a new API Server client
func NewClient(client kubernetes.Interface) Client {
	return clientImpl{
		k8sClient: client,
	}
}

type clientImpl struct {
	k8sClient kubernetes.Interface
}

// GetNodeInfo queries the API server for information about the given node
func (c clientImpl) GetNodeInfo(nodeName string) (*NodeInfo, error) {
	node, err := c.k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not find node information for nodeName='%s'", nodeName)
	}

	return &NodeInfo{
		NodeName:       node.ObjectMeta.Name,
		Labels:         node.Labels,
		Allocatable:    node.Status.Allocatable,
		Capacity:       node.Status.Capacity,
		Conditions:     node.Status.Conditions,
		Unschedulable:  node.Spec.Unschedulable,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
	}, nil
}

// NodeInfo contains information about a specific node
type NodeInfo struct {
	NodeName       string
	Labels         map[string]string
	Allocatable    v1.ResourceList
	Capacity       v1.ResourceList
	Conditions     []v1.NodeCondition
	Unschedulable  bool
	KubeletVersion string
}

// IsMasterNode returns true if the NodeInfo contains the labels that
// identify a node as master.
func (i *NodeInfo) IsMasterNode() bool {
	if val, ok := i.Labels["kubernetes.io/role"]; ok && val == "master" {
		return true
	}
	if _, ok := i.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}
