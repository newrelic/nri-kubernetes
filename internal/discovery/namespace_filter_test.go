package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	testclient "k8s.io/client-go/kubernetes/fake"
)

const namespaceName = "test_namespace"

func TestNamespaceFilterer_IsAllowed(t *testing.T) {
	t.Parallel()

	metrics := definition.RawMetrics{"namespace": namespaceName}

	type testData struct {
		namespaceLabels   labels.Set
		namespaceSelector config.NamespaceSelector
		expected          bool
	}

	testCases := map[string]testData{
		"namespace_allowed_by_default": {
			expected: true,
		},
		"namespace_allowed_with_labels_and_no_selector": {
			namespaceLabels: labels.Set{
				"newrelic.com/scrape": "true",
			},
			expected: true,
		},
		"match_labels_included_namespace_allowed": {
			namespaceLabels: labels.Set{
				"newrelic.com/scrape": "true",
			},
			namespaceSelector: config.NamespaceSelector{
				MatchLabels: map[string]string{
					"newrelic.com/scrape": "true",
				},
			},
			expected: true,
		},
		"match_labels_excluded_namespaces_not_allowed": {
			namespaceLabels: labels.Set{"newrelic.com/scrape": "false"},
			namespaceSelector: config.NamespaceSelector{
				MatchLabels: map[string]string{
					"newrelic.com/scrape": "true",
				},
			},
			expected: false,
		},
		"match_expressions_using_not_in_operator_allow_not_included_namespaces": {
			namespaceLabels: labels.Set{"newrelic.com/scrape": "true"},
			namespaceSelector: config.NamespaceSelector{
				MatchExpressions: []config.Expression{
					{
						Key:      "newrelic.com/scrape",
						Operator: "NotIn",
						Values:   []interface{}{false},
					},
				},
			},
			expected: true,
		},
		"match_expressions_using_not_in_operator_not_allow_included_namespaces": {
			namespaceLabels: labels.Set{"newrelic.com/scrape": "true"},
			namespaceSelector: config.NamespaceSelector{
				MatchExpressions: []config.Expression{
					{
						Key:      "newrelic.com/scrape",
						Operator: "NotIn",
						Values:   []interface{}{true},
					},
				},
			},
			expected: false,
		},
		"match_expressions_using_in_operator_allow_included_namespaces": {
			namespaceLabels: labels.Set{"newrelic.com/scrape": "true"},
			namespaceSelector: config.NamespaceSelector{
				MatchExpressions: []config.Expression{
					{
						Key:      "newrelic.com/scrape",
						Operator: "In",
						Values:   []interface{}{true},
					},
				},
			},
			expected: true,
		},
		"match_expressions_using_in_operator_not_allow_excluded_namespaces": {
			namespaceLabels: labels.Set{"newrelic.com/scrape": "false"},
			namespaceSelector: config.NamespaceSelector{
				MatchExpressions: []config.Expression{
					{
						Key:      "newrelic.com/scrape",
						Operator: "In",
						Values:   []interface{}{"true"},
					},
				},
			},
			expected: false,
		},
		"match_expressions_using_multiple_expressions_allow_included_namespaces": {
			namespaceLabels: labels.Set{"test_label": "1234"},
			namespaceSelector: config.NamespaceSelector{
				MatchExpressions: []config.Expression{
					{
						Key:      "newrelic.com/scrape",
						Operator: "NotIn",
						Values:   []interface{}{"false"},
					},
					{
						Key:      "test_label",
						Operator: "In",
						Values:   []interface{}{1234},
					},
				},
			},
			expected: true,
		},
	}

	for testName, testData := range testCases {
		testData := testData
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			client := testclient.NewSimpleClientset()
			_, err := client.CoreV1().Namespaces().Create(
				context.Background(),
				fakeNamespaceWithNameAndLabels(namespaceName, testData.namespaceLabels),
				metav1.CreateOptions{},
			)
			require.NoError(t, err)

			ns := discovery.NewNamespaceFilter(
				&testData.namespaceSelector,
				client,
				nil,
			)

			t.Cleanup(func() {
				ns.Close()
			})

			require.Equal(t, testData.expected, ns.Match(metrics))
		})
	}
}

func TestNamespaceFilterer_Cache(t *testing.T) {
	t.Parallel()

	metrics := definition.RawMetrics{"namespace": namespaceName}

	type testData struct {
		warmCache func(cache *storer.InMemoryStore)
		prepare   func(nsFilterMock *NamespaceFilterMock)
		assert    func(expected bool, cnsf *discovery.CachedNamespaceFilter)
		expected  bool
	}

	testCases := map[string]testData{
		"namespace_cache_miss_fallback_to_call_informer": {
			warmCache: func(cache *storer.InMemoryStore) {},
			prepare: func(nsFilterMock *NamespaceFilterMock) {
				nsFilterMock.On("Match", metrics).Return(true).Once()
			},
			assert: func(expected bool, cnsf *discovery.CachedNamespaceFilter) {
				require.Equal(t, expected, cnsf.Match(metrics))
			},
			expected: true,
		},
		"namespace_already_in_cache_allowed": {
			warmCache: func(cache *storer.InMemoryStore) {
				cache.Set(namespaceName, true)
			},
			prepare: func(nsFilterMock *NamespaceFilterMock) {
				nsFilterMock.AssertNotCalled(t, "Match")
			},
			assert: func(expected bool, cnsf *discovery.CachedNamespaceFilter) {
				require.Equal(t, expected, cnsf.Match(metrics))
			},
			expected: true,
		},
		"namespace_already_in_cache_not_allowed": {
			warmCache: func(cache *storer.InMemoryStore) {
				cache.Set(namespaceName, false)
			},
			prepare: func(nsFilterMock *NamespaceFilterMock) {
				nsFilterMock.AssertNotCalled(t, "Match")
			},
			assert: func(expected bool, cnsf *discovery.CachedNamespaceFilter) {
				require.Equal(t, expected, cnsf.Match(metrics))
			},
			expected: false,
		},
		"namespace_cache_miss_subsequent_call_uses_cache": {
			warmCache: func(cache *storer.InMemoryStore) {},
			prepare: func(nsFilterMock *NamespaceFilterMock) {
				nsFilterMock.On("Match", metrics).Return(true).Once()
			},
			assert: func(expected bool, cnsf *discovery.CachedNamespaceFilter) {
				require.Equal(t, expected, cnsf.Match(metrics))
				require.Equal(t, expected, cnsf.Match(metrics))
			},
			expected: true,
		},
	}

	for testName, testData := range testCases {
		testData := testData
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			nsFilterMock := newNamespaceFilterMock()

			cache := storer.NewInMemoryStore(storer.DefaultTTL, storer.DefaultInterval, nil)
			testData.warmCache(cache)
			testData.prepare(nsFilterMock)

			cnsf := discovery.NewCachedNamespaceFilter(
				nsFilterMock,
				cache,
			)

			testData.assert(testData.expected, cnsf)

			mock.AssertExpectationsForObjects(t, nsFilterMock)
		})
	}
}

func TestNamespaceFilter_InformerCacheSync(t *testing.T) {
	t.Parallel()

	anotherNamespaceName := "another_namespace"
	client := testclient.NewSimpleClientset()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))

	// Create a namespace with a specific label.
	_, err := client.CoreV1().Namespaces().Create(
		ctx,
		fakeNamespaceWithNameAndLabels(namespaceName, labels.Set{"test_label": "1234"}),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Create the namespace filter.
	ns := discovery.NewNamespaceFilter(
		&config.NamespaceSelector{
			MatchLabels: map[string]string{
				"test_label": "123",
			},
		},
		client,
		nil,
	)
	// Check that recently created namespace is not allowed.
	require.Equal(t, false, ns.Match(definition.RawMetrics{"namespace": namespaceName}))

	t.Cleanup(func() {
		cancel()
		ns.Close()
	})

	// Create a new namespace that can be filtered with the previous given config.
	_, err = client.CoreV1().Namespaces().Create(
		ctx,
		fakeNamespaceWithNameAndLabels(anotherNamespaceName, labels.Set{"test_label": "123"}),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Give some room to the informer to sync, and check that the new namespace is filtered properly.
	err = wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(context.Context) (bool, error) {
		return ns.Match(definition.RawMetrics{"namespace": anotherNamespaceName}), nil
	})
	require.NoError(t, err, "Timed out waiting for the informer to sync")
}

type NamespaceFilterMock struct {
	mock.Mock
}

func newNamespaceFilterMock() *NamespaceFilterMock {
	return &NamespaceFilterMock{}
}

func (ns *NamespaceFilterMock) Match(metrics definition.RawMetrics) bool {
	args := ns.Called(metrics)
	return args.Bool(0)
}

func fakeNamespaceWithNameAndLabels(name string, l labels.Set) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: l,
		},
	}
}
