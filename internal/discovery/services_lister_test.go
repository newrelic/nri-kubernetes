package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	testclient "k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

func Test_services_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	d, _ := discovery.NewServicesLister(client)

	// Discovery with no service
	e, err := d.List(labels.Everything())
	require.NoError(t, err)
	assert.Len(t, e, 0)

	// Discovery after creating a service
	_, err = client.CoreV1().Services("").Create(context.Background(), getFirstService(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.List(labels.Everything())
	require.NoError(t, err)
	assert.Equal(t, []*corev1.Service{getFirstService()}, e)

	// Discovery after deleting such service
	err = client.CoreV1().Services("").Delete(context.Background(), "test", metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.List(labels.Everything())
	require.NoError(t, err)
	assert.Len(t, e, 0)
}

func getFirstService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
}
