package apiserver

import "github.com/pkg/errors"

// TestAPIServer is for testing purposes. It implements the apiserver.Client interface with an in-memory list of objects
type TestAPIServer struct {
	Mem map[string]*NodeInfo
}

func (t TestAPIServer) GetNodeInfo(nodeName string) (*NodeInfo, error) {
	node, ok := t.Mem[nodeName]
	if !ok {
		return nil, errors.New("not found")
	}

	return node, nil
}
