package scrape

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/newrelic/infra-integrations-sdk/data/event"
	"github.com/newrelic/infra-integrations-sdk/data/inventory"
	sdkMetric "github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/internal/testutil"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric/testdata"
	"github.com/newrelic/nri-kubernetes/v3/src/metric"
)

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339, raw)

	return t
}

var expectedEntities = []*integration.Entity{
	{
		Metadata: &integration.EntityMetadata{
			Name:      "test-cluster",
			Namespace: "k8s:cluster",
			IDAttrs:   integration.IDAttributes{},
		},
		Metrics: []*sdkMetric.Set{
			{
				Metrics: map[string]interface{}{
					"event_type":        "K8sClusterSample",
					"clusterName":       "test-cluster",
					"clusterK8sVersion": "v1.15.42",
				},
			},
		},
		Inventory: inventory.New(),
		Events:    []*event.Event{},
	},
	{
		Metadata: &integration.EntityMetadata{
			Name:      "newrelic-infra-rz225",
			Namespace: "k8s:test-cluster:kube-system:pod",
			IDAttrs:   integration.IDAttributes{},
		},
		Metrics: []*sdkMetric.Set{
			{
				Metrics: map[string]interface{}{
					"event_type":                     "K8sPodSample",
					"net.rxBytesPerSecond":           0., // 106175985, but is RATE
					"net.txBytesPerSecond":           0., // 35714359, but is RATE
					"net.errorsPerSecond":            0.,
					"createdAt":                      float64(parseTime("2018-02-14T16:26:33Z").Unix()),
					"startTime":                      float64(parseTime("2018-02-14T16:26:33Z").Unix()),
					"initializedAt":                  float64(parseTime("2018-02-14T16:26:33Z").Unix()),
					"readyAt":                        float64(parseTime("2018-02-27T15:21:18Z").Unix()),
					"scheduledAt":                    float64(parseTime("2018-02-14T16:27:00Z").Unix()),
					"createdKind":                    "DaemonSet",
					"createdBy":                      "newrelic-infra",
					"nodeIP":                         "192.168.99.100",
					"podIP":                          "172.17.0.3",
					"namespace":                      "kube-system",
					"namespaceName":                  "kube-system",
					"nodeName":                       "minikube",
					"podName":                        "newrelic-infra-rz225",
					"daemonsetName":                  "newrelic-infra",
					"isReady":                        float64(1),
					"status":                         "Running",
					"isScheduled":                    float64(1),
					"label.controller-revision-hash": "3887482659",
					"label.name":                     "newrelic-infra",
					"label.pod-template-generation":  "1",
					"displayName":                    "newrelic-infra-rz225", // From entity attributes
					"clusterName":                    "test-cluster",         // From entity attributes
				},
			},
		},
		Inventory: inventory.New(),
		Events:    []*event.Event{},
	},
	{
		Metadata: &integration.EntityMetadata{
			Name:      "newrelic-infra",
			Namespace: "k8s:test-cluster:kube-system:newrelic-infra-rz225:container",
			IDAttrs:   integration.IDAttributes{},
		},
		Metrics: []*sdkMetric.Set{
			{
				Metrics: map[string]interface{}{
					"event_type":                 "K8sContainerSample",
					"memoryUsedBytes":            float64(18083840),
					"memoryWorkingSetBytes":      float64(17113088),
					"memoryUtilization":          float64(17.24609375),
					"cpuUsedCores":               0.01742824,
					"fsAvailableBytes":           float64(14924988416),
					"fsUsedBytes":                float64(126976),
					"fsUsedPercent":              0.0008507538914443524,
					"fsCapacityBytes":            float64(17293533184),
					"fsInodesFree":               float64(9713372),
					"requestedMemoryUtilization": float64(17.24609375),
					"fsInodes":                   float64(9732096),
					"fsInodesUsed":               float64(36),
					"containerName":              "newrelic-infra",
					"containerID":                "69d7203a8f2d2d027ffa51d61002eac63357f22a17403363ef79e66d1c3146b2",
					"containerImage":             "newrelic/ohaik:1.0.0-beta3",
					"containerImageID":           "sha256:1a95d0df2997f93741fbe2a15d2c31a394e752fd942ec29bf16a44163342f6a1",
					"namespace":                  "kube-system",
					"namespaceName":              "kube-system",
					"podName":                    "newrelic-infra-rz225",
					"daemonsetName":              "newrelic-infra",
					"nodeName":                   "minikube",
					"nodeIP":                     "192.168.99.100",
					"restartCount":               float64(6),
					"restartCountDelta":          float64(0), // 0 the first time as it is PDELTA
					"cpuRequestedCores":          0.1,
					"memoryRequestedBytes":       float64(104857600),
					"memoryLimitBytes":           float64(104857600),
					"status":                     "Running",
					"isReady":                    float64(1),
					//"reason":               "",      // TODO ?
					"displayName":                    "newrelic-infra", // From entity attributes
					"clusterName":                    "test-cluster",   // From entity attributes
					"label.controller-revision-hash": "3887482659",
					"label.name":                     "newrelic-infra",
					"label.pod-template-generation":  "1",
					"requestedCpuCoresUtilization":   float64(17.42824),
				},
			},
		},
		Inventory: inventory.New(),
		Events:    []*event.Event{},
	},
}

// We reduce the test fixtures in order to simplify testing.
var kubeletSpecs = definition.SpecGroups{
	"pod":       metric.KubeletSpecs["pod"],
	"container": metric.KubeletSpecs["container"],
}

// grouperMock implements Grouper interface returning mocked metrics which might change over subsequent calls.
type grouperMock struct {
	// ValuesInGroupCalls overwrite some metrics in subsequent calls using {"<path>": {subsequent-values} Examples:
	// {"pod.kube-system_newrelic-infra-rz225.isReady": {0, 1, 2, 3}},
	// {"container.kube-system_newrelic-infra-rz225.restartCount": {0, 1, 2, 3}}
	ValuesInGroupCalls map[string][]interface{}

	groupCallsCount int
}

func (g *grouperMock) Group(definition.SpecGroups) (definition.RawGroups, *data.ErrorGroup) {
	// We reduce the test fixtures in order to simplify testing.
	groupsDefinition := map[string]string{
		"pod":       "kube-system_newrelic-infra-rz225",
		"container": "kube-system_newrelic-infra-rz225_newrelic-infra",
	}
	groups := definition.RawGroups{}
	for entityType, entityName := range groupsDefinition {
		if groups[entityType] == nil {
			groups[entityType] = map[string]definition.RawMetrics{}
		}
		groups[entityType][entityName] = g.metricsForCurrentCall(entityType, entityName)
	}
	g.groupCallsCount++
	return groups, nil
}

// metricsForCurrentCall returns a copy of metrics from `testdata.ExpectedGroupData` corresponding to the provided
// parameters with values changes as configured for the current group call.
func (g *grouperMock) metricsForCurrentCall(entityType, entityName string) definition.RawMetrics {
	metrics := definition.RawMetrics{}
	for k, v := range testdata.ExpectedGroupData[entityType][entityName] {
		metrics[k] = v
	}
	for rawPath, values := range g.ValuesInGroupCalls {
		path := strings.Split(rawPath, ".")
		lenValues := len(values)
		if len(path) != 3 || path[0] != entityType || path[1] != entityName || lenValues < 1 {
			continue
		}
		metricName := path[2]
		valueToUse := g.groupCallsCount % lenValues
		metrics[metricName] = values[valueToUse]
	}
	return metrics
}

func TestPopulateK8s(t *testing.T) {
	t.Parallel()
	intgr := testutil.NewIntegration(t)

	testJob := NewScrapeJob("test", &grouperMock{}, kubeletSpecs)

	k8sVersion := &version.Info{GitVersion: "v1.15.42"}
	errPopulate := testJob.Populate(intgr, "test-cluster", logutil.Debug, k8sVersion)
	assert.Empty(t, errPopulate.Errors)

	expectedInventory := inventory.New()

	err := expectedInventory.SetItem("cluster", "name", expectedEntities[0].Metadata.Name)
	require.NoError(t, err)

	err = expectedInventory.SetItem("cluster", "k8sVersion", k8sVersion.String())
	require.NoError(t, err)

	expectedEntities[0].Inventory = expectedInventory

	require.Equal(t, len(expectedEntities), len(intgr.Entities), "Expected and returned entity lists do not have the same length")

	// Sort slices, so we can later diff them one-by-one for decent readability.
	entitySliceLesser := func(entities []*integration.Entity) func(i, j int) bool {
		return func(i, j int) bool {
			return strings.Compare(entities[i].Metadata.Name, entities[j].Metadata.Name) < 0
		}
	}

	sort.Slice(intgr.Entities, entitySliceLesser(intgr.Entities))
	sort.Slice(expectedEntities, entitySliceLesser(expectedEntities))

	compareIgnoreFields := cmpopts.IgnoreUnexported(integration.Entity{}, integration.EntityMetadata{}, sdkMetric.Set{}, inventory.Inventory{})
	for j := range expectedEntities {
		if diff := cmp.Diff(intgr.Entities[j], expectedEntities[j], compareIgnoreFields); diff != "" {
			t.Errorf("Entities[%d] mismatch: %s", j, diff)
		}
	}
}

func TestRestartCountDeltaValues(t *testing.T) {
	t.Parallel()
	intgr := testutil.NewIntegration(t)

	expectedRestartCountDeltas := []float64{0, 3, 0, 1}

	grouper := &grouperMock{
		ValuesInGroupCalls: map[string][]interface{}{
			"container.kube-system_newrelic-infra-rz225_newrelic-infra.restartCount": {0, 3, 3, 4},
		},
	}
	testJob := NewScrapeJob("test", grouper, kubeletSpecs)

	k8sVersion := &version.Info{GitVersion: "v1.15.42"}
	// Populate data several times to check expected deltas
	for i := 0; i < len(expectedRestartCountDeltas); i++ {
		errPopulate := testJob.Populate(intgr, "test-cluster", logutil.Debug, k8sVersion)
		assert.Empty(t, errPopulate.Errors)
		time.Sleep(time.Second)
	}

	for _, entity := range intgr.Entities {
		if entity.Metadata.Name == "newrelic-infra" {
			lenExpectedDeltas := len(expectedRestartCountDeltas)
			require.Equal(t, lenExpectedDeltas, len(entity.Metrics))
			for i := 0; i < lenExpectedDeltas; i++ {
				assert.Equal(t, expectedRestartCountDeltas[i], entity.Metrics[i].Metrics["restartCountDelta"])
			}
		}
	}
}
