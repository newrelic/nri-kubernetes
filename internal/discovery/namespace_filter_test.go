package discovery_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"
	"github.com/stretchr/testify/require"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	testclient "k8s.io/client-go/kubernetes/fake"
)

const namespaceName = "test_namespace"

func TestNamespaceFilterer_IsAllowed(t *testing.T) {
	t.Parallel()

	type testData struct {
		namespaceLabels   labels.Set
		namespaceSelector config.NamespaceSelector
		expected          bool
	}

	testCases := map[string]testData{
		"namespace_allowed_by_default": {
			expected: true,
		},
		"match_labels_included_namespace_allowed": {
			namespaceLabels: labels.Set{
				"newrelic.com/scrape": "true",
				"ohhh":                "xxx",
			},
			namespaceSelector: config.NamespaceSelector{
				MatchLabels: map[string]string{
					"newrelic.com/scrape": "true",
					"ohhh":                "xxx",
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

			c := config.Config{NamespaceSelector: &testData.namespaceSelector}
			nsFilter := discovery.NewNamespaceFilter(&c, client, newStorerMock(false, persist.ErrNotFound))

			t.Cleanup(func() {
				nsFilter.Close()
			})

			require.Equal(t, testData.expected, nsFilter.IsAllowed(namespaceName))
		})
	}
}

func TestNamespaceFilterer_GetFromCache(t *testing.T) {
	t.Parallel()

	type testData struct {
		namespaceLabels   labels.Set
		namespaceSelector config.NamespaceSelector
		storer            storer.Storer
		expected          bool
	}

	testCases := map[string]testData{
		"namespace_already_in_cache_allowed": {
			namespaceLabels:   labels.Set{},
			namespaceSelector: config.NamespaceSelector{},
			storer:            newStorerMock(true, nil),
			expected:          true,
		},
		"namespace_already_in_cache_not_allowed": {
			namespaceLabels:   labels.Set{},
			namespaceSelector: config.NamespaceSelector{},
			storer:            newStorerMock(false, nil),
			expected:          false,
		},
		"namespace_cache_miss_fallback_to_call_informer": {
			namespaceLabels: labels.Set{
				"newrelic.com/scrape": "true",
			},
			namespaceSelector: config.NamespaceSelector{
				MatchLabels: map[string]string{
					"newrelic.com/scrape": "true",
				},
			},
			storer:   newStorerMock(false, persist.ErrNotFound),
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

			c := config.Config{NamespaceSelector: &testData.namespaceSelector}
			nsFilter := discovery.NewNamespaceFilter(&c, client, testData.storer)

			t.Cleanup(func() {
				nsFilter.Close()
			})

			require.Equal(t, testData.expected, nsFilter.IsAllowed(namespaceName))
		})
	}
}

func TestNamespaceFilterer_SetCache(t *testing.T) {
	t.Parallel()

	storer := storer.NewInMemoryStore(storer.DefaultTTL, storer.DefaultInterval, log.StandardLogger())

	namespaceLabels := labels.Set{"test_label": "1234"}
	namespaceSelector := config.NamespaceSelector{
		MatchLabels: map[string]string{
			"test_label": "1234",
		},
	}

	client := testclient.NewSimpleClientset()
	_, err := client.CoreV1().Namespaces().Create(
		context.Background(),
		fakeNamespaceWithNameAndLabels(namespaceName, namespaceLabels),
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	c := config.Config{NamespaceSelector: &namespaceSelector}
	nsFilter := discovery.NewNamespaceFilter(&c, client, storer)

	t.Cleanup(func() {
		nsFilter.Close()
	})

	var allowed bool
	_, err = storer.Get(namespaceName, &allowed)
	// Empty cache first time, namespace not found.
	require.ErrorIs(t, err, persist.ErrNotFound)

	nsFilter.IsAllowed(namespaceName)

	_, err = storer.Get(namespaceName, &allowed)
	// Cache should be populated with the IsAllowed result.
	require.NoError(t, err)
	require.Equal(t, true, allowed)
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
	nsFilter := discovery.NewNamespaceFilter(&config.Config{
		NamespaceSelector: &config.NamespaceSelector{
			MatchLabels: map[string]string{
				"test_label": "123",
			},
		},
	},
		client,
		newStorerMock(false, persist.ErrNotFound),
	)
	// Check that recently created namespace is not allowed.
	require.Equal(t, false, nsFilter.IsAllowed(namespaceName))

	t.Cleanup(func() {
		cancel()
		nsFilter.Close()
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
		return nsFilter.IsAllowed(anotherNamespaceName), nil
	})
	require.NoError(t, err, "Timed out waiting for the informer to sync")
}

// storerMock implements Grouper interface returning mocked metrics which might change over subsequent calls.
type storerMock struct {
	allowed bool
	err     error

	locker *sync.RWMutex
}

func newStorerMock(allowed bool, err error) *storerMock {
	return &storerMock{
		allowed: allowed,
		err:     err,
		locker:  &sync.RWMutex{},
	}
}

func (s *storerMock) Set(_ string, _ interface{}) int64 {
	return time.Now().Unix()
}

func (s *storerMock) Get(_ string, valuePtr interface{}) (int64, error) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	entry := s.allowed

	// Using reflection to indirectly set the value passed as reference
	varToPopulate := reflect.Indirect(reflect.ValueOf(valuePtr))
	valueToSet := reflect.Indirect(reflect.ValueOf(entry))
	varToPopulate.Set(valueToSet)

	return time.Now().Unix(), s.err
}

func fakeNamespaceWithNameAndLabels(name string, l labels.Set) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: l,
		},
	}
}
