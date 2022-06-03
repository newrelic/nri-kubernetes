package definition_test

import (
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
	"k8s.io/apimachinery/pkg/version"
)

var defaultNS = "playground"

var rawGroupsSample = definition.RawGroups{
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

var specs = definition.SpecGroups{
	"test": definition.SpecGroup{
		TypeGenerator:   fromGroupEntityTypeGuessFunc,
		NamespaceGetter: kubeletMetric.FromLabelGetNamespace(),
		Specs: []definition.Spec{

			{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
			{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
			{
				"metric_3",
				fromMultiple(
					definition.FetchedValues(
						map[string]definition.FetchedValue{
							"multiple_1": "one",
							"multiple_2": "two",
						},
					),
				),
				metric.ATTRIBUTE,
				false,
			},
		},
	},
}

var rawGroupsKSMSample = definition.RawGroups{
	"test": {
		"entity_id_1": definition.RawMetrics{
			"raw_metric_name_1": prometheus.Metric{
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
	},
}

var specsKSM = definition.SpecGroups{
	"test": definition.SpecGroup{
		TypeGenerator:   fromGroupEntityTypeGuessFunc,
		NamespaceGetter: prometheus.FromLabelGetNamespace(),
		Specs: []definition.Spec{
			{"metric_1", prometheus.FromValue("raw_metric_name_1"), metric.GAUGE, false},
			{"metric_2", prometheus.FromValue("raw_metric_name_2"), metric.GAUGE, false},
		},
	},
}

func fromMultiple(values definition.FetchedValues) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		return values, nil
	}
}

// fromGroupMetricSetTypeGuessFunc uses the groupLabel for creating the metric set type sample.
func fromGroupMetricSetTypeGuessFunc(_, groupLabel, _ string, _ definition.RawGroups) (string, error) {
	return fmt.Sprintf("%vSample", strings.Title(groupLabel)), nil
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
		Groups:        rawGroupsSample,
		Specs:         specs,
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
	populator.Groups = rawGroupsKSMSample
	populator.Specs = specsKSM
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

	populated, errs := definition.IntegrationPopulator(testConfig(intgr))
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
				{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.GAUGE, false}, // Source type not correct
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

	populated, errs := definition.IntegrationPopulator(config)
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

	populated, errs := definition.IntegrationPopulator(config)
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

	populated, errs := definition.IntegrationPopulator(config)
	assert.False(t, populated)
	assert.EqualError(t, errs[0], "entity name and type are required when defining one")
	assert.Equal(t, expectedData, intgr.Entities)
}

func TestIntegrationPopulator_MetricsSetsNotPopulated_OnlyEntity(t *testing.T) {
	metricSpecsIncorrect := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{"useless", definition.FromRaw("nonExistentMetric"), metric.GAUGE, false},
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

	populated, errs := definition.IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)

	assert.Contains(t, errs, fmt.Errorf("error populating metric for entity ID entity_id_1: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found"))
	assert.Contains(t, errs, fmt.Errorf("error populating metric for entity ID entity_id_2: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found"))
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_EntityIDGenerator(t *testing.T) {
	generator := func(groupLabel, rawEntityID string, g definition.RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := definition.SpecGroups{
		"test": definition.SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.GAUGE, false},
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

	populated, errs := definition.IntegrationPopulator(config)

	assert.True(t, populated)
	assert.Empty(t, errs)

	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_EntityIDGeneratorFuncWithError(t *testing.T) {
	generator := func(groupLabel, rawEntityID string, g definition.RawGroups) (string, error) {
		return "", fmt.Errorf("error generating entity ID")
	}

	specsWithGeneratorFuncError := definition.SpecGroups{
		"test": definition.SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []definition.Spec{
				{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
			},
		},
	}
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	config := testConfig(intgr)
	config.Specs = specsWithGeneratorFuncError

	populated, errs := definition.IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.Contains(t, errs, fmt.Errorf("error generating entity ID for entity_id_1: error generating entity ID"))
	assert.Contains(t, errs, fmt.Errorf("error generating entity ID for entity_id_2: error generating entity ID"))
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

func TestIntegrationPopulator_PopulateOnlySpecifiedGroups(t *testing.T) {
	generator := func(groupLabel, rawEntityID string, g definition.RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			IDGenerator:   generator,
			Specs: []definition.Spec{
				{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.GAUGE, false},
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

	populated, errs := definition.IntegrationPopulator(config)

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
		return "", fmt.Errorf("error generating entity type")
	}

	specsWithGeneratorFuncError := definition.SpecGroups{
		"test": definition.SpecGroup{
			TypeGenerator: generatorWithError,
			Specs: []definition.Spec{
				{"metric_1", definition.FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", definition.FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	config := testConfig(intgr)
	config.Specs = specsWithGeneratorFuncError

	populated, errs := definition.IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.Contains(t, errs, fmt.Errorf("error generating entity type for entity_id_1: error generating entity type"))
	assert.Contains(t, errs, fmt.Errorf("error generating entity type for entity_id_2: error generating entity type"))
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

func TestIntegrationPopulator_msTypeGuesserFuncWithError(t *testing.T) {
	msTypeGuesserFuncWithError := func(_, groupLabel, _ string, _ definition.RawGroups) (string, error) {
		return "", fmt.Errorf("error setting event type")
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	expectedEntityData1, err := intgr.Entity("entity_id_1", "playground:test")
	require.NoError(t, err)

	expectedEntityData2, err := intgr.Entity("entity_id_2", "playground:test")
	require.NoError(t, err)

	config := testConfig(intgr)
	config.MsTypeGuesser = msTypeGuesserFuncWithError

	populated, errs := definition.IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.Contains(t, errs, fmt.Errorf("error setting event type"))
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_MetricFormatFilterNamespace(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	populated, errs := definition.IntegrationPopulator(testConfigMetricFormatWithFilterer(intgr))

	assert.True(t, populated)
	// Only cluster entity and nsB
	assert.Equal(t, 2, len(intgr.Entities))
	assert.Empty(t, errs)
}

func TestIntegrationPopulator_PrometheusFormatFilterNamespace(t *testing.T) {
	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	populated, errs := definition.IntegrationPopulator(testConfigPrometheusFormatWithFilterer(intgr))

	assert.True(t, populated)
	// Only cluster entity and nsB entities
	assert.Equal(t, 3, len(intgr.Entities))
	assert.Empty(t, errs)
}

type NamespaceFilterMock struct{}

func (nf NamespaceFilterMock) IsAllowed(namespace string) bool {
	if namespace == "nsA" {
		return false
	}
	return true
}
