package client

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)



func TestCheckConnection(t *testing.T) {
	GetClientFromRestInterface=
	_, err := getClientFromRestInterface(fake.NewSimpleClientset())
	assert.NoError(t, err)
}
