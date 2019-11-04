package apiserver

import "fmt"

// TestAPIServer is for testing purposes. It implements the apiserver.Client interface with an in-memory list of objects
type TestAPIServer struct {
	Mem map[string]*NodeInfo
}

func (t TestAPIServer) GetNodeInfo(nodeName string) (*NodeInfo, error) {
	node, ok := t.Mem[nodeName]
	if !ok {
		return nil, fmt.Errorf("could not find node info for: %s", nodeName)
	}

	return node, nil
}
