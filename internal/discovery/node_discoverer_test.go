package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
)

func Test_nodes_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	d, closeChan := discovery.NewNodesGetter(client)

	defer close(closeChan)

	// Discovery with no node
	e, err := d.Get("test-node")
	require.Error(t, err)

	// Discovery after creating a node
	_, err = client.CoreV1().Nodes().Create(context.Background(), getFirstNode(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get("first-node")
	require.NoError(t, err)
	assert.Equal(t, getFirstNode(), e)

	// Discovery after deleting such node
	err = client.CoreV1().Nodes().Delete(context.Background(), "first-node", metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get("first-node")
	require.Error(t, err)
}

func getFirstNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "first-node",
		},
	}
}
