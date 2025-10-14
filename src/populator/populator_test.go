//nolint:paralleltest // Some tests intentionally do not use t.Parallel or use it in subtests only.
package populator

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/newrelic/infra-integrations-sdk/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	kubeletMetric "github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/version"
)

const defaultNS = "playground"

var (
	errTestGenerateID        = errors.New("error generating entity ID")
	errTestGenerateType      = errors.New("error generating entity type")
	errTestSettingEventType  = errors.New("error setting event type")
	errTestIDGeneratorFailed = errors.New("id generator failed")
)

func getRawGroupsSample() definition.RawGroups {
	return definition.RawGroups{
		"test": {
			"entity_id_1": definition.RawMetrics{
				"raw_metric_name_1": 1,
				"raw_metric_name_2": "metric_value_2",
				"raw_metric_name_3": map[string]interface{}{
					"foo": "bar",
				},
				"namespace": "nsA",
			},
			"entity_id_2": definition.RawMetrics{
				"raw_metric_name_1": 2,
				"raw_metric_name_2": "metric_value_4",
				"raw_metric_name_3": map[string]interface{}{
					"foo": "bar",
				},
				"namespace": "nsB",
			},
		},
	}
}

func getSpecsSample() definition.SpecGroups {
	return definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator:   fromGroupEntityTypeGuessFunc,
			NamespaceGetter: kubeletMetric.FromLabelGetNamespace,
			Specs: []definition.Spec{
				{
					Name:      "metric_1",
					ValueFunc: definition.FromRaw("raw_metric_name_1"),
					Type:      metric.GAUGE,
					Optional:  false,
				},
				{
					Name:      "metric_2",
					ValueFunc: definition.FromRaw("raw_metric_name_2"),
					Type:      metric.ATTRIBUTE,
					Optional:  false,
				},
				{
					Name: "metric_3",
					ValueFunc: fromMultiple(
						definition.FetchedValues(
							map[string]definition.FetchedValue{
								"multiple_1": "one",
								"multiple_2": "two",
							},
						),
					),
					Type:     metric.ATTRIBUTE,
					Optional: false,
				},
			},
		},
	}
}

func getRawGroupsKSMSample() definition.RawGroups {
	return definition.RawGroups{
		"test": {
			"entity_id_1": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value:  prometheus.CounterValue(1),
					Labels: map[string]string{},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsA",
					},
				},
			},
			"entity_id_2": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsA",
					},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsA",
					},
				},
			},
			"entity_id_3": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
			},
			"entity_id_4": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
			},
		},
		definition.NamespaceGroup: {
			"entity_id_filtered": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsA",
					},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsA",
					},
				},
			},
			"entity_id_not_filtered": definition.RawMetrics{
				"raw_metric_name_1": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
				"raw_metric_name_2": prometheus.Metric{
					Value: prometheus.CounterValue(1),
					Labels: map[string]string{
						"namespace": "nsB",
					},
				},
			},
		},
	}
}

func getSpecsKSM() definition.SpecGroups {
	return definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator:   fromGroupEntityTypeGuessFunc,
			NamespaceGetter: prometheus.FromLabelGetNamespace,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: prometheus.FromValue("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: prometheus.FromValue("raw_metric_name_2"), Type: metric.GAUGE, Optional: false},
			},
		},
		definition.NamespaceGroup: definition.SpecGroup{
			TypeGenerator:   fromGroupEntityTypeGuessFunc,
			NamespaceGetter: prometheus.FromLabelGetNamespace,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: prometheus.FromValue("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: prometheus.FromValue("raw_metric_name_2"), Type: metric.GAUGE, Optional: false},
			},
		},
	}
}

func fromMultiple(values definition.FetchedValues) definition.FetchFunc {
	return func(_ string, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
		return values, nil
	}
}

// fromGroupMetricSetTypeGuessFunc uses the groupLabel for creating the metric set type sample.
func fromGroupMetricSetTypeGuessFunc(groupLabel string) (string, error) {
	return fmt.Sprintf("%vSample", cases.Title(language.Und).String(groupLabel)), nil
}

func fromGroupEntityTypeGuessFunc(groupLabel string, _ string, _ definition.RawGroups, prefix string) (string, error) {
	return fmt.Sprintf("%s:%s", prefix, groupLabel), nil
}

func testConfig(i *integration.Integration) *definition.IntegrationPopulateConfig {
	return &definition.IntegrationPopulateConfig{
		Integration:   i,
		ClusterName:   defaultNS,
		K8sVersion:    &version.Info{GitVersion: "v1.15.42"},
		MsTypeGuesser: fromGroupMetricSetTypeGuessFunc,
		Groups:        getRawGroupsSample(),
		Specs:         getSpecsSample(),
	}
}

func testConfigMetricFormatWithFilterer(i *integration.Integration) *definition.IntegrationPopulateConfig {
	populator := testConfig(i)
	populator.Filterer = NamespaceFilterMock{}
	return populator
}

func testConfigPrometheusFormatWithFilterer(i *integration.Integration) *definition.IntegrationPopulateConfig {
	populator := testConfig(i)
	populator.Filterer = NamespaceFilterMock{}
	populator.Groups = getRawGroupsKSMSample()
	populator.Specs = getSpecsKSM()
	return populator
}

func TestIntegrationPopulator_CorrectValue(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("entity_id_1", "playground:test")
	require.NoError(t, err)

	expectedMetricSet1 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    1,
			"metric_2":    "metric_value_2",
			"multiple_1":  "one",
			"multiple_2":  "two",
			"displayName": "entity_id_1",
			"clusterName": "playground",
		},
	}
	expectedEntityData1.Metrics = []*metric.Set{expectedMetricSet1}

	expectedEntityData2, err := intgr.Entity("entity_id_2", "playground:test")
	require.NoError(t, err)

	expectedMetricSet2 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    2,
			"metric_2":    "metric_value_4",
			"multiple_1":  "one",
			"multiple_2":  "two",
			"displayName": "entity_id_2",
			"clusterName": "playground",
		},
	}
	expectedEntityData2.Metrics = []*metric.Set{expectedMetricSet2}

	populated, errs := IntegrationPopulator(testConfig(intgr))
	assert.True(t, populated)
	assert.Empty(t, errs)
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_PartialResult(t *testing.T) {
	metricSpecsWithIncompatibleType := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: definition.FromRaw("raw_metric_name_2"), Type: metric.GAUGE, Optional: false}, // Source type not correct
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("entity_id_1", "playground:test")
	require.NoError(t, err)

	expectedMetricSet1 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    1,
			"displayName": "entity_id_1",
			"clusterName": "playground",
		},
	}
	expectedEntityData1.Metrics = []*metric.Set{expectedMetricSet1}

	expectedEntityData2, err := intgr.Entity("entity_id_2", "playground:test")
	require.NoError(t, err)

	expectedMetricSet2 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    2,
			"displayName": "entity_id_2",
			"clusterName": "playground",
		},
	}
	expectedEntityData2.Metrics = []*metric.Set{expectedMetricSet2}

	config := testConfig(intgr)
	config.Specs = metricSpecsWithIncompatibleType

	populated, errs := IntegrationPopulator(config)
	assert.True(t, populated)
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)

	assert.Len(t, errs, 2)
}

func TestIntegrationPopulator_EntitiesDataNotPopulated_EmptyMetricGroups(t *testing.T) {
	metricGroupEmpty := definition.RawGroups{}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedData := make([]*integration.Entity, 0)

	config := testConfig(intgr)
	config.Groups = metricGroupEmpty

	populated, errs := IntegrationPopulator(config)
	assert.False(t, populated)
	assert.Nil(t, errs)
	assert.Equal(t, expectedData, intgr.Entities)
}

func TestIntegrationPopulator_EntitiesDataNotPopulated_ErrorSettingEntities(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	metricGroupEmptyEntityID := definition.RawGroups{
		"test": {
			"": definition.RawMetrics{
				"raw_metric_name_1": 1,
				"raw_metric_name_2": "metric_value_2",
				"raw_metric_name_3": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	expectedData := []*integration.Entity{}

	config := testConfig(intgr)
	config.Groups = metricGroupEmptyEntityID

	populated, errs := IntegrationPopulator(config)
	assert.False(t, populated)
	assert.EqualError(t, errs[0], "entity name and type are required when defining one")
	assert.Equal(t, expectedData, intgr.Entities)
}

func TestIntegrationPopulator_MetricsSetsNotPopulated_OnlyEntity(t *testing.T) {
	metricSpecsIncorrect := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{Name: "useless", ValueFunc: definition.FromRaw("nonExistentMetric"), Type: metric.GAUGE, Optional: false},
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("entity_id_1", "playground:test")
	require.NoError(t, err)

	expectedMetricSet1 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"displayName": "entity_id_1",
			"clusterName": "playground",
		},
	}
	expectedEntityData1.Metrics = []*metric.Set{expectedMetricSet1}

	expectedEntityData2, err := intgr.Entity("entity_id_2", "playground:test")
	require.NoError(t, err)

	expectedMetricSet2 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"displayName": "entity_id_2",
			"clusterName": "playground",
		},
	}
	expectedEntityData2.Metrics = []*metric.Set{expectedMetricSet2}

	config := testConfig(intgr)
	config.Specs = metricSpecsIncorrect

	populated, errs := IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)

	// Define the expected error strings.
	expectedErrorStrings := []string{
		"error populating metric for entity ID entity_id_1: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found",
		"error populating metric for entity ID entity_id_2: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found",
	}

	actualErrorStrings := make([]string, len(errs))
	for i, err := range errs {
		actualErrorStrings[i] = err.Error()
	}

	// Use ElementsMatch to compare the contents of the slices, ignoring order.
	assert.ElementsMatch(t, expectedErrorStrings, actualErrorStrings)
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_EntityIDGenerator(t *testing.T) {
	generator := func(_, rawEntityID string, _ definition.RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := definition.SpecGroups{
		"test": definition.SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: definition.FromRaw("raw_metric_name_2"), Type: metric.GAUGE, Optional: false},
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	raw := definition.RawGroups{
		"test": {
			"testEntity1": {
				"raw_metric_name_1": 1,
				"raw_metric_name_2": 2,
			},
			"testEntity2": {
				"raw_metric_name_1": 3,
				"raw_metric_name_2": 4,
			},
		},
	}

	expectedEntityData1, err := intgr.Entity("testEntity1-generated", "playground:test")
	require.NoError(t, err)

	expectedMetricSet1 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    1,
			"metric_2":    2,
			"displayName": "testEntity1-generated",
			"clusterName": "playground",
		},
	}
	expectedEntityData1.Metrics = []*metric.Set{expectedMetricSet1}

	expectedEntityData2, err := intgr.Entity("testEntity2-generated", "playground:test")
	require.NoError(t, err)

	expectedMetricSet2 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    3,
			"metric_2":    4,
			"displayName": "testEntity2-generated",
			"clusterName": "playground",
		},
	}
	expectedEntityData2.Metrics = []*metric.Set{expectedMetricSet2}

	config := testConfig(intgr)
	config.Groups = raw
	config.Specs = withGeneratorSpec

	populated, errs := IntegrationPopulator(config)

	assert.True(t, populated)
	assert.Empty(t, errs)

	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_EntityIDGeneratorFuncWithError(t *testing.T) {
	generator := func(_, _ string, _ definition.RawGroups) (string, error) {
		return "", errTestGenerateID
	}

	specsWithGeneratorFuncError := definition.SpecGroups{
		"test": definition.SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: definition.FromRaw("raw_metric_name_2"), Type: metric.ATTRIBUTE, Optional: false},
			},
		},
	}
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	config := testConfig(intgr)
	config.Specs = specsWithGeneratorFuncError

	populated, errs := IntegrationPopulator(config)

	expectedErr1 := "could not generate entity ID for entity_id_1: error generating entity ID"
	expectedErr2 := "could not generate entity ID for entity_id_2: error generating entity ID"

	errStrings := make([]string, len(errs))
	for i, err := range errs {
		errStrings[i] = err.Error()
	}

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.Contains(t, errStrings, expectedErr1)
	assert.Contains(t, errStrings, expectedErr2)
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

//nolint:funlen
func TestIntegrationPopulator_PopulateOnlySpecifiedGroups(t *testing.T) {
	generator := func(_, rawEntityID string, _ definition.RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			IDGenerator:   generator,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: definition.FromRaw("raw_metric_name_2"), Type: metric.GAUGE, Optional: false},
			},
		},
	}

	groups := definition.RawGroups{
		"test": {
			"testEntity11": {
				"raw_metric_name_1": 1,
				"raw_metric_name_2": 2,
			},
			"testEntity12": {
				"raw_metric_name_1": 3,
				"raw_metric_name_2": 4,
			},
		},
		"test2": {
			"testEntity21": {
				"raw_metric_name_1": 5,
				"raw_metric_name_2": 6,
			},
			"testEntity22": {
				"raw_metric_name_1": 7,
				"raw_metric_name_2": 8,
			},
		},
	}

	// Create a dummy integration, used only to create entities easily.
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("testEntity11-generated", "playground:test")
	require.NoError(t, err)

	expectedMetricSet1 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    float64(1),
			"metric_2":    float64(2),
			"displayName": "testEntity11-generated",
			"clusterName": "playground",
		},
	}
	expectedEntityData1.Metrics = []*metric.Set{expectedMetricSet1}

	expectedEntityData2, err := intgr.Entity("testEntity12-generated", "playground:test")
	require.NoError(t, err)

	expectedMetricSet2 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":  "TestSample",
			"metric_1":    float64(3),
			"metric_2":    float64(4),
			"displayName": "testEntity12-generated",
			"clusterName": "playground",
		},
	}
	expectedEntityData2.Metrics = []*metric.Set{expectedMetricSet2}

	expectedEntityData3, err := intgr.Entity("playground", "k8s:cluster")
	require.NoError(t, err)

	expectedMetricSet3 := &metric.Set{
		Metrics: map[string]interface{}{
			"event_type":        "K8sClusterSample",
			"clusterName":       "playground",
			"clusterK8sVersion": "v1.15.42",
		},
	}
	expectedEntityData3.Metrics = []*metric.Set{expectedMetricSet3}
	expectedInventory := inventory.New()

	err = expectedInventory.SetItem("cluster", "name", "playground")
	require.NoError(t, err)

	err = expectedInventory.SetItem("cluster", "k8sVersion", "v1.15.42")
	require.NoError(t, err)

	expectedEntityData3.Inventory = expectedInventory

	intgr.Clear()

	config := testConfig(intgr)
	config.Specs = withGeneratorSpec
	config.Groups = groups

	populated, errs := IntegrationPopulator(config)

	assert.True(t, populated)
	assert.Empty(t, errs)
	assert.Len(t, intgr.Entities, 3)

	compareIgnoreFields := cmpopts.IgnoreUnexported(integration.Entity{}, integration.EntityMetadata{}, metric.Set{}, inventory.Inventory{})
	for _, expectedEntity := range []*integration.Entity{
		expectedEntityData1,
		expectedEntityData2,
		expectedEntityData3,
	} {
		found := false
		closestMatchDiff := ""
		for _, entity := range intgr.Entities {
			curDiff := cmp.Diff(entity, expectedEntity, compareIgnoreFields)
			// If curDiff is empty we got an exact match, return.
			if curDiff == "" {
				found = true
				break
			}

			// Otherwise, store current diff as closest match if it is smaller than the closestMatch, or if the latter is empty
			if closestMatchDiff == "" || len(curDiff) < len(closestMatchDiff) {
				closestMatchDiff = curDiff
			}
		}

		if !found {
			t.Fatalf("Entity list does not contain %q. Closest match:\n%s", expectedEntity.Metadata.Name, closestMatchDiff)
		}
	}
}

func TestIntegrationPopulator_EntityTypeGeneratorFuncWithError(t *testing.T) {
	generatorWithError := func(_ string, _ string, _ definition.RawGroups, _ string) (string, error) {
		return "", errTestGenerateType
	}

	specsWithGeneratorFuncError := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: generatorWithError,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
				{Name: "metric_2", ValueFunc: definition.FromRaw("raw_metric_name_2"), Type: metric.ATTRIBUTE, Optional: false},
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	config := testConfig(intgr)
	config.Specs = specsWithGeneratorFuncError

	populated, errs := IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	for _, err := range errs {
		assert.ErrorIs(t, err, ErrGenerateType)
	}
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

func TestIntegrationPopulator_msTypeGuesserFuncWithError(t *testing.T) {
	msTypeGuesserFuncWithError := func(_ string) (string, error) {
		return "", errTestSettingEventType
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("entity_id_1", "playground:test")
	require.NoError(t, err)

	expectedEntityData2, err := intgr.Entity("entity_id_2", "playground:test")
	require.NoError(t, err)

	config := testConfig(intgr)
	config.MsTypeGuesser = msTypeGuesserFuncWithError

	populated, errs := IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.ErrorIs(t, errs[0], errTestSettingEventType)
	assert.ErrorIs(t, errs[1], errTestSettingEventType)
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_MetricFormatFilterNamespace(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	populated, errs := IntegrationPopulator(testConfigMetricFormatWithFilterer(intgr))

	assert.True(t, populated)
	// Only cluster entity and nsB
	assert.Equal(t, 2, len(intgr.Entities))
	assert.Empty(t, errs)
}

func TestIntegrationPopulator_PrometheusFormatFilterNamespace(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	populated, errs := IntegrationPopulator(testConfigPrometheusFormatWithFilterer(intgr))

	assert.True(t, populated)
	// Only cluster entity, nsB test entities and namespace group entities
	assert.Equal(t, 5, len(intgr.Entities))
	assert.Empty(t, errs)

	// Check for extraAttributes
	for _, entity := range intgr.Entities {
		// Namespace Entity should include filtered label
		if entity.Metrics[0].Metrics["event_type"] == "NamespaceSample" {
			if entity.Metrics[0].Metrics["displayName"] == "entity_id_filtered" {
				assert.Equal(t, "true", entity.Metrics[0].Metrics[definition.NamespaceFilteredLabel])
			} else {
				assert.Equal(t, "false", entity.Metrics[0].Metrics[definition.NamespaceFilteredLabel])
			}
		}
		if entity.Metrics[0].Metrics["event_type"] != "K8sClusterSample" {
			assert.NotEmpty(t, entity.Metrics[0].Metrics["displayName"])
		}
		assert.NotEmpty(t, entity.Metrics[0].Metrics["clusterName"])
	}
}

func TestIntegrationPopulator_CustomMsTypeGuesser(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	customMsTypeGuesser := func(_ string) (string, error) {
		return "Custom", nil
	}

	config := testConfig(intgr)
	config.Specs = definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator:   fromGroupEntityTypeGuessFunc,
			NamespaceGetter: kubeletMetric.FromLabelGetNamespace,
			MsTypeGuesser:   customMsTypeGuesser,
			Specs: []definition.Spec{
				{Name: "metric_1", ValueFunc: definition.FromRaw("raw_metric_name_1"), Type: metric.GAUGE, Optional: false},
			},
		},
	}

	populated, errs := IntegrationPopulator(config)
	assert.True(t, populated)
	assert.Empty(t, errs)
	for _, e := range intgr.Entities {
		if e.Metadata.Namespace == "k8s:cluster" { // the custom MsTypeGuesser does not apply to the cluster entity.
			continue
		}
		for _, ms := range e.Metrics {
			assert.Equal(t, "Custom", ms.Metrics["event_type"])
		}
	}
}

func TestIntegrationPopulator_IntegrationVersionInInventory(t *testing.T) {
	integrationVersion := "2.3.1"
	intgr, err := integration.New("com.newrelic.kubernetes", integrationVersion, integration.InMemoryStore())
	require.NoError(t, err)
	config := testConfig(intgr)

	expectedInventory := inventory.New()
	err = expectedInventory.SetItem("cluster", "name", defaultNS)
	require.NoError(t, err)
	err = expectedInventory.SetItem("cluster", "k8sVersion", config.K8sVersion.String())
	require.NoError(t, err)
	err = expectedInventory.SetItem("cluster", "newrelic.integrationVersion", integrationVersion)
	require.NoError(t, err)
	err = expectedInventory.SetItem("cluster", "newrelic.integrationName", "com.newrelic.kubernetes")
	require.NoError(t, err)

	populated, errs := IntegrationPopulator(config)
	assert.True(t, populated)
	assert.Empty(t, errs)
	assert.Equal(t, intgr.Entities[2].Inventory, expectedInventory)
}

//nolint:funlen
func TestPrepareProcessingUnits(t *testing.T) {
	// --- Mock Data and Specs for the test ---

	// Mock Spec for a simple, single-entity group.
	singleEntitySpec := definition.SpecGroup{
		IDGenerator: func(_, rawEntityID string, _ definition.RawGroups) (string, error) {
			return fmt.Sprintf("%s-generated", rawEntityID), nil
		},
		TypeGenerator: func(_ string, _ string, _ definition.RawGroups, _ string) (string, error) {
			return "k8s:test:single", nil
		},
	}

	// Mock Spec for a group that needs to be split.
	splitEntitySpec := definition.SpecGroup{
		TypeGenerator: func(_ string, _ string, _ definition.RawGroups, _ string) (string, error) {
			return "k8s:test:resourcequota", nil
		},
		SplitByLabel:    "resource",
		SliceMetricName: "kube_resourcequota",
	}

	// Mock Spec with a failing IDGenerator.
	idGeneratorFailsSpec := definition.SpecGroup{
		IDGenerator: func(_, _ string, _ definition.RawGroups) (string, error) {
			return "", errTestIDGeneratorFailed
		},
		TypeGenerator: fromGroupEntityTypeGuessFunc,
	}

	// Mock RawMetrics for the split group.
	splitRawMetrics := definition.RawMetrics{
		"kube_resourcequota_created": prometheus.Metric{Value: prometheus.GaugeValue(123)},
		"kube_resourcequota": []prometheus.Metric{
			{Labels: prometheus.Labels{"resource": "pods", "type": "hard"}, Value: prometheus.GaugeValue(10)},
			{Labels: prometheus.Labels{"resource": "secrets", "type": "hard"}, Value: prometheus.GaugeValue(20)},
			{Labels: prometheus.Labels{"resource": "pods", "type": "used"}, Value: prometheus.GaugeValue(5)},
		},
	}

	// --- Test Cases ---

	testCases := []struct {
		name          string
		groupLabel    string
		entityID      string
		rawMetrics    definition.RawMetrics
		specGroup     definition.SpecGroup
		expectedUnits int
		expectedErr   string // Defines expected errors.
		assertFunc    func(t *testing.T, units []processingUnit)
	}{
		{
			name:          "Single_Entity_Path",
			groupLabel:    "test",
			entityID:      "single-entity-01",
			rawMetrics:    definition.RawMetrics{"metric1": 1},
			specGroup:     singleEntitySpec,
			expectedUnits: 1,
			assertFunc: func(t *testing.T, units []processingUnit) {
				t.Helper()
				assert.Equal(t, "single-entity-01-generated", units[0].entityID)
				assert.Equal(t, "k8s:test:single", units[0].entityType)
				assert.Equal(t, 1, units[0].rawMetrics["metric1"])
			},
		},
		{
			name:          "Split_By_Label_Path",
			groupLabel:    "resourcequota",
			entityID:      "parent_entity_id",
			rawMetrics:    splitRawMetrics,
			specGroup:     splitEntitySpec,
			expectedUnits: 2, // Should split into "pods" and "secrets".
			assertFunc: func(t *testing.T, units []processingUnit) {
				t.Helper()
				// Find the "pods" sub-unit for detailed inspection.
				var podsUnit processingUnit
				for _, u := range units {
					if strings.HasSuffix(u.entityID, "_pods") {
						podsUnit = u
						break
					}
				}
				require.NotNil(t, podsUnit.rawMetrics, "pods unit not found")
				assert.Equal(t, "parent_entity_id_pods", podsUnit.entityID)
				assert.Equal(t, "k8s:test:resourcequota", podsUnit.entityType)

				// Check that shared metrics were copied.
				assert.Contains(t, podsUnit.rawMetrics, "kube_resourcequota_created")

				// Check that the slice was correctly filtered (should only have 2 pod metrics).
				podMetrics, ok := podsUnit.rawMetrics["kube_resourcequota"].([]prometheus.Metric)
				require.True(t, ok)
				assert.Len(t, podMetrics, 2)
				assert.Equal(t, "pods", podMetrics[0].Labels["resource"])
				assert.Equal(t, "pods", podMetrics[1].Labels["resource"])
			},
		},
		{
			name:       "Error_when_split_metric_is_not_a_slice",
			groupLabel: "resourcequota",
			entityID:   "parent_entity_id",
			rawMetrics: definition.RawMetrics{
				"kube_resourcequota": "this is a string, not a slice",
			},
			specGroup:     splitEntitySpec,
			expectedUnits: 0,
			expectedErr:   "requires a slice of metrics",
		},
		{
			name:          "Error_on_IDGenerator_failure",
			groupLabel:    "test",
			entityID:      "single-entity-01",
			rawMetrics:    definition.RawMetrics{"metric1": 1},
			specGroup:     idGeneratorFailsSpec,
			expectedUnits: 0,
			expectedErr:   "id generator failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &definition.IntegrationPopulateConfig{
				Groups: definition.RawGroups{
					tc.groupLabel: {
						tc.entityID: tc.rawMetrics,
					},
				},
				Specs: definition.SpecGroups{
					tc.groupLabel: tc.specGroup,
				},
			}

			// --- Execute the function under test ---
			units, err := prepareProcessingUnits(config, tc.groupLabel, tc.entityID, tc.rawMetrics)

			// --- Assertions ---
			if tc.expectedErr != "" {
				require.Error(t, err, "Expected an error but got none")
				assert.Contains(t, err.Error(), tc.expectedErr, "Error message did not contain expected text")
				return // Stop this test case.
			}

			// If no error was expected.
			require.NoError(t, err, "Did not expect an error")
			require.Len(t, units, tc.expectedUnits)
			if tc.assertFunc != nil {
				tc.assertFunc(t, units)
			}
		})
	}
}

func TestPopulateCluster(t *testing.T) {
	// --- 1. Setup ---
	const (
		clusterName        = "test-cluster"
		k8sVersionStr      = "v1.25.0"
		integrationName    = "com.newrelic.kubernetes"
		integrationVersion = "3.0.0"
	)

	// Create an in-memory integration object for the test.
	intgr, err := integration.New(integrationName, integrationVersion, integration.InMemoryStore())
	require.NoError(t, err, "Unexpected error while creating integration object")

	k8sVersion := mockVersion{version: k8sVersionStr}

	// --- 2. Execute the function under test ---
	err = populateCluster(intgr, clusterName, k8sVersion)

	// --- 3. Assertions ---

	// The function should execute without error.
	require.NoError(t, err, "populateCluster returned an unexpected error")

	// Verify that exactly one entity was created in the integration payload.
	require.Len(t, intgr.Entities, 1, "Expected exactly one entity to be created")
	clusterEntity := intgr.Entities[0]

	// Verify the entity's core metadata.
	assert.Equal(t, clusterName, clusterEntity.Metadata.Name, "Entity name is incorrect")
	assert.Equal(t, "k8s:cluster", clusterEntity.Metadata.Namespace, "Entity type (namespace) is incorrect")

	// Verify the inventory data.
	require.NotNil(t, clusterEntity.Inventory, "Entity inventory should not be nil")
	clusterInventory := clusterEntity.Inventory.Items()
	require.Contains(t, clusterInventory, "cluster", "Inventory is missing 'cluster' category")

	assert.Equal(t, clusterName, clusterInventory["cluster"]["name"])
	assert.Equal(t, k8sVersionStr, clusterInventory["cluster"]["k8sVersion"])
	assert.Equal(t, integrationName, clusterInventory["cluster"]["newrelic.integrationName"])
	assert.Equal(t, integrationVersion, clusterInventory["cluster"]["newrelic.integrationVersion"])

	// Verify the metric set data.
	require.Len(t, clusterEntity.Metrics, 1, "Expected exactly one metric set on the entity")
	metricSet := clusterEntity.Metrics[0]
	assert.Equal(t, "K8sClusterSample", metricSet.Metrics["event_type"])
	assert.Equal(t, clusterName, metricSet.Metrics["clusterName"])
	assert.Equal(t, k8sVersionStr, metricSet.Metrics["clusterK8sVersion"])
}

func TestMetricSetPopulate_SkipsNilValues(t *testing.T) {
	// 1. Setup
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	// Create a spec with one metric that is required, and one that will return nil.
	specs := definition.SpecGroups{
		"test": {
			Specs: []definition.Spec{
				{
					Name: "good_metric",
					ValueFunc: func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
						return "good_value", nil
					},
					Type: metric.ATTRIBUTE,
				},
				{
					Name: "nil_metric",
					ValueFunc: func(_, _ string, _ definition.RawGroups) (definition.FetchedValue, error) {
						//nolint:nilnil
						return nil, nil
					},
					Type:     metric.ATTRIBUTE,
					Optional: false, // Mark as not optional to ensure no error is generated.
				},
			},
		},
	}

	// Create an entity and a metric set to populate.
	e, _ := intgr.Entity("test-entity", "k8s:test")
	ms := e.NewMetricSet("TestSample")
	groups := definition.RawGroups{"test": {"test-entity": {}}}

	// 2. Execute
	populated, errs := metricSetPopulate(ms, "test", "test-entity", groups, specs)

	// 3. Assert
	assert.True(t, populated, "Expected populated to be true because one metric was set")
	assert.Empty(t, errs, "Expected no errors because nil value should be skipped")

	// Verify that only the non-nil metric was added to the metric set.
	assert.Equal(t, "good_value", ms.Metrics["good_metric"])
	assert.NotContains(t, ms.Metrics, "nil_metric")
}

func TestIntegrationPopulator_WithCrossGroupDependency2(t *testing.T) {
	// Spec for a "pod" that needs to look up its "service" to generate a full entity ID.
	podSpecWithDependency := definition.SpecGroup{
		TypeGenerator: fromGroupEntityTypeGuessFunc, // Generates "playground:pod"
		IDGenerator: func(groupLabel, entityID string, groups definition.RawGroups) (string, error) {
			// This generator requires access to the "service" group.
			serviceGroup, ok := groups["service"]
			if !ok {
				return "", errors.New("service group not found")
			}

			// Find the service and append its name.
			for serviceID, metrics := range serviceGroup {
				if name, ok := metrics["name"]; ok && name == "my-service" {
					// Generate the ID using info from another group.
					return fmt.Sprintf("%s_service-is-%s", entityID, serviceID), nil
				}
			}

			return "", errors.New("could not find related service")
		},
		Specs: []definition.Spec{
			{Name: "podName", ValueFunc: definition.FromRaw("name"), Type: metric.ATTRIBUTE},
		},
	}

	serviceSpec := definition.SpecGroup{
		TypeGenerator: fromGroupEntityTypeGuessFunc,
		Specs: []definition.Spec{
			{Name: "serviceName", ValueFunc: definition.FromRaw("name"), Type: metric.ATTRIBUTE},
		},
	}

	// Mock data
	crossGroupData := definition.RawGroups{
		"pod": {
			"my-pod-123": {
				"name": "my-pod-123",
			},
		},
		"service": {
			"my-service-abc": {
				"name": "my-service",
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	config := &definition.IntegrationPopulateConfig{
		Integration:   intgr,
		ClusterName:   defaultNS,
		K8sVersion:    &version.Info{GitVersion: "v1.15.42"},
		MsTypeGuesser: fromGroupMetricSetTypeGuessFunc,
		Groups:        crossGroupData,
		Specs: definition.SpecGroups{
			"pod":     podSpecWithDependency,
			"service": serviceSpec,
		},
	}

	populated, errs := IntegrationPopulator(config)

	assert.True(t, populated, "Expected the integration to be populated")
	assert.Empty(t, errs, "Expected no errors during population")
	require.Len(t, intgr.Entities, 3, "Expected three entities (pod, service, and cluster)")

	var podEntity *integration.Entity
	for _, e := range intgr.Entities {
		if strings.HasPrefix(e.Metadata.Name, "my-pod-123") {
			podEntity = e
			break
		}
	}

	require.NotNil(t, podEntity, "Pod entity was not found in the integration payload")
	assert.Equal(t, "my-pod-123_service-is-my-service-abc", podEntity.Metadata.Name)
}

type NamespaceFilterMock struct{}

func (nf NamespaceFilterMock) IsAllowed(namespace string) bool {
	return namespace != "nsA"
}

type mockVersion struct {
	version string
}

func (m mockVersion) String() string {
	return m.version
}
