package apiserver

import (
	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/pkg/errors"
)

// Client an interface for querying the k8s API server
type Client interface {
	GetNodeInfo(nodeName string) (*NodeInfo, error)
}

// NewClient creates a new API Server client
func NewClient(client client.Kubernetes) Client {
	return clientImpl{
		k8sClient: client,
	}
}

type clientImpl struct {
	k8sClient client.Kubernetes
}

// GetNodeInfo queries the API server for information about the given node
func (c clientImpl) GetNodeInfo(nodeName string) (*NodeInfo, error) {

	node, err := c.k8sClient.FindNode(nodeName)

	if err != nil {
		return nil, errors.Wrapf(err, "could not find node information for nodeName='%s'", nodeName)
	}

	return &NodeInfo{
		NodeName: node.ObjectMeta.Name,
		Labels:   node.Labels,
	}, nil
}

// NodeInfo contains information about a specific node
type NodeInfo struct {
	NodeName string
	Labels   map[string]string
}

func (i *NodeInfo) IsMasterNode() bool {
	if val, ok := i.Labels["kubernetes.io/role"]; ok && val == "master" {
		return true
	}
	if _, ok := i.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}
