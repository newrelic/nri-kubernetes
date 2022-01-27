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

const (
	testNamespace = "testNamespace"
	podName       = "testPod"
)

var multiLabelSelector = labels.Set{
	"foo": "matching",
	"bar": "matching",
}

var labelSelector = labels.Set{
	"baz": "matching",
}

func Test_pods_lister_returns(t *testing.T) {
	t.Parallel()

	type testData struct {
		namespace string
		selector  labels.Selector
		result    []*corev1.Pod
	}

	testCases := map[string]testData{
		"pod_when_selector_matches": {
			"",
			labels.SelectorFromSet(labelSelector),
			[]*corev1.Pod{getPodUniqueLabelSelector()},
		},
		"pod_when_selector_and_namespace_match": {
			testNamespace,
			labels.SelectorFromSet(labelSelector),
			[]*corev1.Pod{getPodUniqueLabelSelector()},
		},
		"pod_when_multilabels_match": {
			"",
			labels.SelectorFromSet(multiLabelSelector),
			[]*corev1.Pod{getPodMultiLabelSelector()},
		},
		"pod_when_multilabels_partially_match": {
			testNamespace,
			labels.SelectorFromSet(labels.Set{
				"foo": multiLabelSelector["foo"],
			}),
			[]*corev1.Pod{getPodMultiLabelSelector()},
		},
		"no_pod_when_namespace_not_match": {
			"not-matching",
			labels.SelectorFromSet(labelSelector),
			nil,
		},
		"no_pod_when_labels_no_match": {
			testNamespace,
			labels.SelectorFromSet(labels.Set{"not-matching": "label"}),
			nil,
		},
		"no_pod_when_partial_multilabel_no_match": {
			testNamespace,
			labels.SelectorFromSet(labels.Set{
				"baz":          labelSelector["baz"],
				"not-matching": "label",
			}),
			nil,
		},
	}

	client := testclient.NewSimpleClientset()

	for _, pod := range getPods() {
		_, err := client.CoreV1().Pods(testNamespace).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	time.Sleep(time.Second)

	for testName, testData := range testCases {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			podListerer, closeChan := discovery.NewNamespacePodListerer(
				discovery.PodListererConfig{
					Namespaces: []string{testData.namespace},
					Client:     client,
				},
			)

			podLister, ok := podListerer.Lister(testData.namespace)
			require.True(t, ok)

			pods, err := podLister.List(testData.selector)
			require.NoError(t, err)
			assert.Equal(t, testData.result, pods)
			close(closeChan)
		})
	}
}

func Test_pod_multi_namespace_discovery(t *testing.T) {
	t.Parallel()

	differentNamespace := "differentNamespace"
	labelSelectorFoo := labels.Set{
		"foo": "matching",
	}

	client := testclient.NewSimpleClientset(
		getPodUniqueLabelSelector(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: differentNamespace,
				Labels:    labelSelectorFoo,
			},
		},
	)

	podListerer, closeChan := discovery.NewNamespacePodListerer(
		discovery.PodListererConfig{
			Namespaces: []string{testNamespace, differentNamespace},
			Client:     client,
		},
	)

	defer close(closeChan)

	t.Run("get_pod_from_testNamespace", func(t *testing.T) {
		t.Parallel()

		pl, ok := podListerer.Lister(testNamespace)
		require.True(t, ok)

		pods, err := pl.List(labels.SelectorFromSet(labelSelector))
		require.NoError(t, err)
		assert.Len(t, pods, 1)
	})

	t.Run("get_pod_from_differentNamespace", func(t *testing.T) {
		t.Parallel()

		pl, ok := podListerer.Lister(differentNamespace)
		require.True(t, ok)

		pods, err := pl.List(labels.SelectorFromSet(labelSelectorFoo))
		require.NoError(t, err)
		assert.Len(t, pods, 1)
	})
}

func Test_pods_lister_updates(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	podListerer, closeChan := discovery.NewNamespacePodListerer(
		discovery.PodListererConfig{
			Client:     client,
			Namespaces: []string{testNamespace},
		},
	)

	defer close(closeChan)

	podLister, ok := podListerer.Lister(testNamespace)
	require.True(t, ok)

	// List with no pod
	pods, err := podLister.List(labels.Everything())
	require.NoError(t, err)
	require.Nil(t, pods)

	// List after creating a pod
	_, err = client.CoreV1().Pods(testNamespace).Create(
		context.Background(),
		getPodUniqueLabelSelector(),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)

	pods, err = podLister.List(labels.Everything())
	require.NoError(t, err)
	require.Len(t, pods, 1)

	// List after deleting such pod
	err = client.CoreV1().Pods(testNamespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	// List with no pod
	pods, err = podLister.List(labels.Everything())
	require.NoError(t, err)
	require.Nil(t, pods)
}

func Test_pods_lister_stop_channel(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	podListerer, closeChan := discovery.NewNamespacePodListerer(
		discovery.PodListererConfig{
			Client:     client,
			Namespaces: []string{testNamespace},
		},
	)

	close(closeChan)

	// List after creating a pod
	_, err := client.CoreV1().Pods(testNamespace).Create(
		context.Background(),
		getPodUniqueLabelSelector(),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)

	podLister, ok := podListerer.Lister(testNamespace)
	require.True(t, ok)

	pods, err := podLister.List(labels.Everything())
	require.NoError(t, err)
	require.Nil(t, pods)
}

func getPodUniqueLabelSelector() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: testNamespace,
			Labels:    labelSelector,
		},
	}
}

func getPodMultiLabelSelector() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podMultiLabel",
			Namespace: testNamespace,
			Labels:    multiLabelSelector,
		},
	}
}

func getPodNoSelector() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podNoSelector",
			Namespace: testNamespace,
		},
	}
}

func getPods() []*corev1.Pod {
	return []*corev1.Pod{
		getPodUniqueLabelSelector(),
		getPodMultiLabelSelector(),
		getPodNoSelector(),
	}
}
