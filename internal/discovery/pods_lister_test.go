package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	testclient "k8s.io/client-go/kubernetes/fake"
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
		config   discovery.PodsListerConfig
		selector labels.Selector
		result   []*corev1.Pod
	}

	testCases := map[string]testData{
		"pod_when_selector_matches": {
			discovery.PodsListerConfig{},
			labels.SelectorFromSet(labelSelector),
			[]*corev1.Pod{getPodUniqueLabelSelector()},
		},
		"pod_when_selector_and_namespace_match": {
			discovery.PodsListerConfig{Namespace: testNamespace},
			labels.SelectorFromSet(labelSelector),
			[]*corev1.Pod{getPodUniqueLabelSelector()},
		},
		"pod_when_multilabels_match": {
			discovery.PodsListerConfig{},
			labels.SelectorFromSet(multiLabelSelector),
			[]*corev1.Pod{getPodMultiLabelSelector()},
		},
		"pod_when_multilabels_partially_match": {
			discovery.PodsListerConfig{Namespace: testNamespace},
			labels.SelectorFromSet(labels.Set{
				"foo": multiLabelSelector["foo"],
			}),
			[]*corev1.Pod{getPodMultiLabelSelector()},
		},
		"no_pod_when_namespace_not_match": {
			discovery.PodsListerConfig{Namespace: "not-matching"},
			labels.SelectorFromSet(labelSelector),
			nil,
		},
		"no_pod_when_labels_no_match": {
			discovery.PodsListerConfig{Namespace: testNamespace},
			labels.SelectorFromSet(labels.Set{"not-matching": "label"}),
			nil,
		},
		"no_pod_when_partial_multilabellabels_no_match": {
			discovery.PodsListerConfig{Namespace: testNamespace},
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

			testData.config.Client = client

			podLister, closeChan := discovery.NewPodsLister(testData.config)

			pods, err := podLister.List(testData.selector)
			require.NoError(t, err)
			assert.Equal(t, testData.result, pods)
			close(closeChan)
		})
	}
}

func Test_pods_lister_updates(t *testing.T) {
	t.Parallel()

	client := testclient.NewSimpleClientset()
	podLister, closeChan := discovery.NewPodsLister(
		discovery.PodsListerConfig{
			Client:    client,
			Namespace: testNamespace,
		},
	)

	defer close(closeChan)

	// List with no pod
	pods, err := podLister.List(labels.Everything())
	require.NoError(t, err)
	require.Nil(t, pods)

	// List after creating a pod
	_, err = client.CoreV1().Pods(testNamespace).Create(context.Background(), getPodUniqueLabelSelector(), metav1.CreateOptions{})
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
	podLister, closeChan := discovery.NewPodsLister(discovery.PodsListerConfig{Client: client})

	close(closeChan)

	// List after creating a pod
	_, err := client.CoreV1().Pods(testNamespace).Create(context.Background(), getPodUniqueLabelSelector(), metav1.CreateOptions{})
	require.NoError(t, err)
	time.Sleep(time.Second)

	pods, err := podLister.List(labels.Everything())
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
