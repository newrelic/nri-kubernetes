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

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

const nodeName = "name"

func Test_nodes_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	d, closeChan := discovery.NewNodeLister(client)

	defer close(closeChan)

	// Discovery with no node
	e, err := d.Get(nodeName)
	require.Error(t, err)
	require.Nil(t, e)

	// Discovery after creating a node
	_, err = client.CoreV1().Nodes().Create(context.Background(), fakeNode(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get(nodeName)
	require.NoError(t, err)
	assert.Equal(t, fakeNode(), e)

	// Discovery after deleting such node
	err = client.CoreV1().Nodes().Delete(context.Background(), nodeName, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get(nodeName)
	require.Error(t, err)
}

func Test_nodes_stop_channel(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	d, closeChan := discovery.NewNodeLister(client)

	close(closeChan)

	// Discovery after creating a node with closed channel
	_, err := client.CoreV1().Nodes().Create(context.Background(), fakeNode(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	// Discovery with closed informer
	e, err := d.Get(nodeName)
	require.Error(t, err)
	require.Nil(t, e)
}

func fakeNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
}
