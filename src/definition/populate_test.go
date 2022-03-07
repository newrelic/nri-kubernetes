package definition

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/newrelic/infra-integrations-sdk/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
)

var defaultNS = "playground"

var rawGroupsSample = RawGroups{
	"test": {
		"entity_id_1": RawMetrics{
			"raw_metric_name_1": 1,
			"raw_metric_name_2": "metric_value_2",
			"raw_metric_name_3": map[string]interface{}{
				"foo": "bar",
			},
		},
		"entity_id_2": RawMetrics{
			"raw_metric_name_1": 2,
			"raw_metric_name_2": "metric_value_4",
			"raw_metric_name_3": map[string]interface{}{
				"foo": "bar",
			},
		},
	},
}

var specs = SpecGroups{
	"test": SpecGroup{
		TypeGenerator: fromGroupEntityTypeGuessFunc,
		Specs: []Spec{

			{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
			{"metric_2", FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
			{
				"metric_3",
				fromMultiple(
					FetchedValues(
						map[string]FetchedValue{
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

func fromMultiple(values FetchedValues) FetchFunc {
	return func(groupLabel, entityID string, groups RawGroups) (FetchedValue, error) {
		return values, nil
	}
}

// fromGroupMetricSetTypeGuessFunc uses the groupLabel for creating the metric set type sample.
func fromGroupMetricSetTypeGuessFunc(_, groupLabel, _ string, _ RawGroups) (string, error) {
	return fmt.Sprintf("%vSample", strings.Title(groupLabel)), nil
}

func fromGroupEntityTypeGuessFunc(groupLabel string, _ string, _ RawGroups, prefix string) (string, error) {
	return fmt.Sprintf("%s:%s", prefix, groupLabel), nil
}

func testConfig(i *integration.Integration) *IntegrationPopulateConfig {
	return &IntegrationPopulateConfig{
		Integration:   i,
		ClusterName:   defaultNS,
		K8sVersion:    &version.Info{GitVersion: "v1.15.42"},
		MsTypeGuesser: fromGroupMetricSetTypeGuessFunc,
		Groups:        rawGroupsSample,
		Specs:         specs,
	}
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
	metricSpecsWithIncompatibleType := SpecGroups{
		"test": SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []Spec{
				{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", FromRaw("raw_metric_name_2"), metric.GAUGE, false}, // Source type not correct
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
	metricGroupEmpty := RawGroups{}

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

	metricGroupEmptyEntityID := RawGroups{
		"test": {
			"": RawMetrics{
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
	metricSpecsIncorrect := SpecGroups{
		"test": SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []Spec{
				{"useless", FromRaw("nonExistentMetric"), metric.GAUGE, false},
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

	assert.Contains(t, errs, fmt.Errorf("error populating metric for entity ID entity_id_1: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found"))
	assert.Contains(t, errs, fmt.Errorf("error populating metric for entity ID entity_id_2: cannot fetch value for metric \"useless\": metric \"nonExistentMetric\" not found"))
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}

func TestIntegrationPopulator_EntityIDGenerator(t *testing.T) {
	generator := func(groupLabel, rawEntityID string, g RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := SpecGroups{
		"test": SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []Spec{
				{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", FromRaw("raw_metric_name_2"), metric.GAUGE, false},
			},
		},
	}

	intgr, err := integration.New("nr.test", "1.0.0", integration.InMemoryStore())
	require.NoError(t, err)

	raw := RawGroups{
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
	generator := func(groupLabel, rawEntityID string, g RawGroups) (string, error) {
		return "", fmt.Errorf("error generating entity ID")
	}

	specsWithGeneratorFuncError := SpecGroups{
		"test": SpecGroup{
			IDGenerator:   generator,
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			Specs: []Spec{
				{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
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
	assert.Contains(t, errs, fmt.Errorf("error generating entity ID for entity_id_1: error generating entity ID"))
	assert.Contains(t, errs, fmt.Errorf("error generating entity ID for entity_id_2: error generating entity ID"))
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

func TestIntegrationPopulator_PopulateOnlySpecifiedGroups(t *testing.T) {
	generator := func(groupLabel, rawEntityID string, g RawGroups) (string, error) {
		return fmt.Sprintf("%v-generated", rawEntityID), nil
	}

	withGeneratorSpec := SpecGroups{
		"test": SpecGroup{
			TypeGenerator: fromGroupEntityTypeGuessFunc,
			IDGenerator:   generator,
			Specs: []Spec{
				{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", FromRaw("raw_metric_name_2"), metric.GAUGE, false},
			},
		},
	}

	groups := RawGroups{
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
	generatorWithError := func(_ string, _ string, _ RawGroups, _ string) (string, error) {
		return "", fmt.Errorf("error generating entity type")
	}

	specsWithGeneratorFuncError := SpecGroups{
		"test": SpecGroup{
			TypeGenerator: generatorWithError,
			Specs: []Spec{
				{"metric_1", FromRaw("raw_metric_name_1"), metric.GAUGE, false},
				{"metric_2", FromRaw("raw_metric_name_2"), metric.ATTRIBUTE, false},
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
	assert.Contains(t, errs, fmt.Errorf("error generating entity type for entity_id_1: error generating entity type"))
	assert.Contains(t, errs, fmt.Errorf("error generating entity type for entity_id_2: error generating entity type"))
	assert.Equal(t, intgr.Entities, []*integration.Entity{})
}

func TestIntegrationPopulator_msTypeGuesserFuncWithError(t *testing.T) {
	msTypeGuesserFuncWithError := func(_, groupLabel, _ string, _ RawGroups) (string, error) {
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

	populated, errs := IntegrationPopulator(config)

	assert.False(t, populated)
	assert.Len(t, errs, 2)
	assert.Contains(t, errs, fmt.Errorf("error setting event type"))
	assert.Contains(t, intgr.Entities, expectedEntityData1)
	assert.Contains(t, intgr.Entities, expectedEntityData2)
}
