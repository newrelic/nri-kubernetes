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

const (
	secretName         = "name"
	secretNamespace    = "namespace"
	differentNamespace = "abcd"
)

func Test_secrets_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()

	listerer, closeChan := discovery.NewNamespaceSecretListerer(
		discovery.SecretListererConfig{
			Namespaces: []string{secretNamespace},
			Client:     client,
		},
	)

	defer close(closeChan)

	d, ok := listerer.Lister(secretNamespace)
	require.True(t, ok)

	// Discovery with no secret
	e, err := d.Get(secretName)
	require.Error(t, err)
	require.Nil(t, e)

	// Discovery after creating a secret
	_, err = client.CoreV1().Secrets(secretNamespace).Create(
		context.Background(),
		fakeSecret(secretNamespace),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err = d.Get(secretName)
	require.NoError(t, err)
	assert.Equal(t, fakeSecret(secretNamespace), e)

	// Discovery after deleting such secret
	err = client.CoreV1().Secrets(secretNamespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	_, err = d.Get(secretName)
	require.Error(t, err)
}

func Test_secrets_multi_namespace_discovery(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(
		fakeSecret(secretNamespace),
		fakeSecret(differentNamespace),
	)

	listerer, _ := discovery.NewNamespaceSecretListerer(
		discovery.SecretListererConfig{
			Namespaces: []string{secretNamespace, differentNamespace},
			Client:     client,
		},
	)

	t.Run("get_secrets_from_secretNamespace", func(t *testing.T) {
		t.Parallel()

		d, ok := listerer.Lister(secretNamespace)
		require.True(t, ok)

		e, err := d.Get(secretName)
		require.NoError(t, err)
		assert.Equal(t, secretName, e.Name)
	})

	t.Run("get_secrets_from_differentNamespace", func(t *testing.T) {
		t.Parallel()

		d, ok := listerer.Lister(differentNamespace)
		require.True(t, ok)

		e, err := d.Get(secretName)
		require.NoError(t, err)
		assert.Equal(t, secretName, e.Name)
	})
}

func Test_secrets_ignores_different_namespaces(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: differentNamespace,
		},
	})

	listerer, _ := discovery.NewNamespaceSecretListerer(
		discovery.SecretListererConfig{
			Namespaces: []string{secretNamespace},
			Client:     client,
		},
	)

	d, ok := listerer.Lister(secretNamespace)
	require.True(t, ok)

	e, err := d.Get(secretName)
	require.Error(t, err)
	assert.Nil(t, e)
}

func Test_secrets_stop_channel(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()

	listerer, closeChan := discovery.NewNamespaceSecretListerer(
		discovery.SecretListererConfig{
			Namespaces: []string{secretNamespace, differentNamespace},
			Client:     client,
		},
	)

	d, ok := listerer.Lister(secretNamespace)
	require.True(t, ok)

	sl, ok := listerer.Lister(differentNamespace)
	require.True(t, ok)

	close(closeChan)

	// Discovery after creating a secret will fail since we stopped the channel
	_, err := client.CoreV1().Secrets(secretNamespace).Create(
		context.Background(),
		fakeSecret(secretNamespace),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
	_, err = client.CoreV1().Secrets(differentNamespace).Create(
		context.Background(),
		fakeSecret(differentNamespace),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)

	e, err := d.Get(secretName)
	require.Error(t, err)
	assert.Nil(t, e)

	e, err = sl.Get(secretName)
	require.Error(t, err)
	assert.Nil(t, e)
}

func Test_informer_does_not_hit_multiple_times_backend(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset(fakeSecret(secretNamespace))

	listerer, _ := discovery.NewNamespaceSecretListerer(
		discovery.SecretListererConfig{
			Namespaces: []string{secretNamespace},
			Client:     client,
		},
	)

	d, ok := listerer.Lister(secretNamespace)
	require.True(t, ok)

	_, err := d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)
	_, err = d.Get(secretName)
	assert.Nil(t, err)

	actions := client.Actions()

	var counterList, counterGet int

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

func fakeSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"testData": []byte("testData"),
		},
	}
}
