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

const (
	secretName         = "name"
	secretNamespace    = "namespace"
	differentNamespace = "abcd"
)

func Test_secrets_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()

	d, closeChan := discovery.NewSecretNamespaceLister(
		discovery.SecretListerConfig{
			Namespace: secretNamespace,
			Client:    client,
		},
	)

	defer close(closeChan)

	// Discovery with no secret
	e, err := d.Get(secretName)
	require.Error(t, err)
	require.Nil(t, e)

	// Discovery after creating a secret
	_, err = client.CoreV1().Secrets(secretNamespace).Create(context.Background(), fakeSecret(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get(secretName)
	require.NoError(t, err)
	assert.Equal(t, fakeSecret(), e)

	// Discovery after deleting such secret
	err = client.CoreV1().Secrets(secretNamespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get(secretName)
	require.Error(t, err)
}

func Test_secrets_ignores_different_namespaces(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: differentNamespace,
		},
	})

	d, _ := discovery.NewSecretNamespaceLister(
		discovery.SecretListerConfig{
			Namespace: secretNamespace,
			Client:    client,
		},
	)

	e, err := d.Get(secretName)
	require.Error(t, err)
	assert.Nil(t, e)
}

func Test_secrets_stop_channel(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(fakeSecret())

	d, closeChan := discovery.NewSecretNamespaceLister(
		discovery.SecretListerConfig{
			Namespace: secretNamespace,
			Client:    client,
		},
	)

	close(closeChan)

	e, err := d.Get(secretName)
	require.Error(t, err)
	assert.Nil(t, e)
}

func Test_informer_does_not_hit_multiple_times_backend(t *testing.T) {
	t.Parallel()

	var err error
	client := testclient.NewSimpleClientset(fakeSecret())

	d, _ := discovery.NewSecretNamespaceLister(
		discovery.SecretListerConfig{
			Namespace: secretNamespace,
			Client:    client,
		},
	)

	_, err = d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)

	actions := client.Actions()

	var counterList int
	var counterGet int
	for _, a := range actions {
		if a.GetVerb() == "list" {
			counterList++
		}
		if a.GetVerb() == "get" {
			counterGet++
		}
	}
	assert.Equal(t, 1, counterList)
	assert.Equal(t, 0, counterGet)

}

func fakeSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		Data: map[string][]byte{
			"testData": []byte("testData"),
		},
	}
}
