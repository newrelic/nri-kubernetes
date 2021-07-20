package metric

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/newrelic/infra-integrations-sdk/data/event"
	"github.com/newrelic/infra-integrations-sdk/data/inventory"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/src/data"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
	"github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric/testdata"
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
		Metrics: []*metric.Set{
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
		Metrics: []*metric.Set{
			{
				Metrics: map[string]interface{}{
					"event_type":                     "K8sPodSample",
					"net.rxBytesPerSecond":           0., // 106175985, but is RATE
					"net.txBytesPerSecond":           0., // 35714359, but is RATE
					"net.errorsPerSecond":            0.,
					"createdAt":                      float64(parseTime("2018-02-14T16:26:33Z").Unix()),
					"startTime":                      float64(parseTime("2018-02-14T16:26:33Z").Unix()),
					"createdKind":                    "DaemonSet",
					"createdBy":                      "newrelic-infra",
					"nodeIP":                         "192.168.99.100",
					"namespace":                      "kube-system",
					"namespaceName":                  "kube-system",
					"nodeName":                       "minikube",
					"podName":                        "newrelic-infra-rz225",
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
		Metrics: []*metric.Set{
			{
				Metrics: map[string]interface{}{
					"event_type":            "K8sContainerSample",
					"memoryUsedBytes":       float64(18083840),
					"memoryWorkingSetBytes": float64(17113088),
					"cpuUsedCores":          0.01742824,
					"fsAvailableBytes":      float64(14924988416),
					"fsUsedBytes":           float64(126976),
					"fsUsedPercent":         0.0008507538914443524,
					"fsCapacityBytes":       float64(17293533184),
					"fsInodesFree":          float64(9713372),
					"fsInodes":              float64(9732096),
					"fsInodesUsed":          float64(36),
					"containerName":         "newrelic-infra",
					"containerID":           "69d7203a8f2d2d027ffa51d61002eac63357f22a17403363ef79e66d1c3146b2",
					"containerImage":        "newrelic/ohaik:1.0.0-beta3",
					"containerImageID":      "sha256:1a95d0df2997f93741fbe2a15d2c31a394e752fd942ec29bf16a44163342f6a1",
					"namespace":             "kube-system",
					"namespaceName":         "kube-system",
					"podName":               "newrelic-infra-rz225",
					"nodeName":              "minikube",
					"nodeIP":                "192.168.99.100",
					"restartCount":          float64(6),
					"cpuRequestedCores":     0.1,
					"memoryRequestedBytes":  float64(104857600),
					"memoryLimitBytes":      float64(104857600),
					"status":                "Running",
					"isReady":               float64(1),
					//"reason":               "",      // TODO ?
					"displayName":                    "newrelic-infra", // From entity attributes
					"clusterName":                    "test-cluster",   // From entity attributes
					"label.controller-revision-hash": "3887482659",
					"label.name":                     "newrelic-infra",
					"label.pod-template-generation":  "1",
				},
			},
		},
		Inventory: inventory.New(),
		Events:    []*event.Event{},
	},
}

// We reduce the test fixtures in order to simplify testing.
var kubeletSpecs = definition.SpecGroups{
	"pod":       KubeletSpecs["pod"],
	"container": KubeletSpecs["container"],
}

func TestPopulateK8s(t *testing.T) {
	p := NewK8sPopulator()

	intgr, err := integration.New("test", "test")
	assert.NoError(t, err)
	intgr.Clear()

	// We reduce the test fixtures in order to simplify testing.
	foo := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-rz225": testdata.ExpectedGroupData["pod"]["kube-system_newrelic-infra-rz225"],
		},
		"container": {
			"kube-system_newrelic-infra-rz225_newrelic-infra": testdata.ExpectedGroupData["container"]["kube-system_newrelic-infra-rz225_newrelic-infra"],
		},
	}

	k8sVersion := &version.Info{GitVersion: "v1.15.42"}
	err = p.Populate(foo, kubeletSpecs, intgr, "test-cluster", k8sVersion)
	require.IsType(t, err, data.PopulateResult{})
	assert.Empty(t, err.(data.PopulateResult).Errors)

	expectedInventory := inventory.New()

	err = expectedInventory.SetItem("cluster", "name", expectedEntities[0].Metadata.Name)
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

	compareIgnoreFields := cmpopts.IgnoreUnexported(integration.Entity{}, metric.Set{}, inventory.Inventory{})
	for j := range expectedEntities {
		if diff := cmp.Diff(intgr.Entities[j], expectedEntities[j], compareIgnoreFields); diff != "" {
			t.Errorf("Entities[%d] mismatch: %s", j, diff)
		}
	}
}
