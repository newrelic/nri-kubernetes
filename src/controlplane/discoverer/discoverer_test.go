package discoverer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane/discoverer"
)

func Test_Discoverer_does_not_fail(t *testing.T) {
	t.Parallel()

	namespace := "testNamespace"
	nodeName := "testNode"
	selector := "foo=bar"

	testCases := []struct {
		name         string
		pods         []*corev1.Pod
		autodiscover config.AutodiscoverControlPlane
		assert       func(*testing.T, *corev1.Pod, error)
	}{
		// Discover returns nil error and matching pod cases.
		{
			name: "when_single_pod_match_in_the_same_node",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  selector,
				MatchNode: true,
			},
			pods: []*corev1.Pod{newPod("foo", namespace, selector, nodeName)},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.NoError(t, err)
				assert.Equal(t, newPod("foo", namespace, selector, nodeName), p)
			},
		},
		{
			name: "when_a_pod_matches_but_is_in_different_node_with_matchnode_false",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  selector,
				MatchNode: false,
			},
			pods: []*corev1.Pod{newPod("foo", namespace, selector, "otherNode")},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.NoError(t, err)
				assert.Equal(t, newPod("foo", namespace, selector, "otherNode"), p)
			},
		},
		{
			name: "when_multiple_pods_match",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  selector,
				MatchNode: true,
			},
			pods: []*corev1.Pod{
				newPod("foo", namespace, selector, nodeName),
				newPod("bar", namespace, selector, nodeName),
				newPod("baz", namespace, selector, nodeName),
			},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.NoError(t, err)
				assert.Contains(t, []string{"foo", "bar", "baz"}, p.Name)
			},
		},
		{
			name: "when_empty_selector",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  "",
				MatchNode: true,
			},
			pods: []*corev1.Pod{newPod("foo", namespace, selector, nodeName)},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.NoError(t, err)
				assert.Equal(t, newPod("foo", namespace, selector, nodeName), p)
			},
		},
		// Discover returns nil pod and nil error cases.
		{
			name: "when_a_pod_matches_but_is_in_different_node_with_matchnode_true",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  selector,
				MatchNode: true,
			},
			pods: []*corev1.Pod{newPod("foo", namespace, selector, "otherNode")},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.ErrorIs(t, err, discoverer.ErrPodNotFound)
				assert.Nil(t, p)
			},
		},
		{
			name: "when_no_pod_matches_selector",
			autodiscover: config.AutodiscoverControlPlane{
				Namespace: namespace,
				Selector:  "not-matching=selector",
				MatchNode: true,
			},
			pods: []*corev1.Pod{newPod("foo", namespace, selector, nodeName)},
			assert: func(t *testing.T, p *corev1.Pod, err error) {
				assert.ErrorIs(t, err, discoverer.ErrPodNotFound)
				assert.Nil(t, p)
			},
		},
	}

	for _, tc := range testCases {
		test := tc

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			k8sClient := fake.NewSimpleClientset()

			for _, pod := range test.pods {
				_, err := k8sClient.CoreV1().Pods(pod.Namespace).Create(
					context.Background(),
					pod,
					metav1.CreateOptions{},
				)
				require.NoError(t, err)
			}

			pl, _ := discovery.NewNamespacePodListerer(
				discovery.PodListererConfig{
					Client:     k8sClient,
					Namespaces: []string{test.autodiscover.Namespace},
				},
			)
			pd, err := discoverer.New(
				discoverer.Config{
					PodListerer: pl,
					NodeName:    nodeName,
				},
			)
			require.NoError(t, err)

			pod, err := pd.Discover(test.autodiscover)
			test.assert(t, pod, err)
		})
	}
}

func Test_Discoverer_fails(t *testing.T) {
	t.Parallel()

	t.Run("when_no_lister_is_found_for_the_namespace", func(t *testing.T) {
		t.Parallel()

		pl, _ := discovery.NewNamespacePodListerer(
			discovery.PodListererConfig{
				Client:     fake.NewSimpleClientset(),
				Namespaces: []string{"foo"},
			},
		)
		pd, err := discoverer.New(discoverer.Config{PodListerer: pl})
		require.NoError(t, err)

		_, err = pd.Discover(config.AutodiscoverControlPlane{
			Namespace: "missing-namespace",
		})
		assert.Error(t, err)
	})

	t.Run("when_selector_is_invalid", func(t *testing.T) {
		t.Parallel()

		pl, _ := discovery.NewNamespacePodListerer(
			discovery.PodListererConfig{
				Client:     fake.NewSimpleClientset(),
				Namespaces: []string{"foo"},
			},
		)
		pd, err := discoverer.New(discoverer.Config{PodListerer: pl})
		require.NoError(t, err)

		_, err = pd.Discover(config.AutodiscoverControlPlane{
			Namespace: "foo",
			Selector:  "=invalid=selector=",
		})
		assert.Error(t, err)
	})
}

func newPod(name, namespace, selector, node string) *corev1.Pod {
	labelsSet, _ := labels.ConvertSelectorToLabelsMap(selector)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labelsSet,
		},
		Spec: corev1.PodSpec{
			NodeName: node,
		},
	}
}
