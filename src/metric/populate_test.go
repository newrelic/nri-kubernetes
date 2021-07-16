package metric

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/newrelic/infra-integrations-sdk/data/event"
	"sort"
	"strings"
	"testing"
	"time"

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
					"entityName":        "k8s:cluster:test-cluster",
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

					"entityName":                     "k8s:test-cluster:kube-system:pod:newrelic-infra-rz225",
					"event_type":                     "K8sPodSample",
					"net.rxBytesPerSecond":           0., // 106175985, but is RATE
					"net.txBytesPerSecond":           0., // 35714359, but is RATE
					"net.errorsPerSecond":            0.,
					"createdAt":                      parseTime("2018-02-14T16:26:33Z").Unix(),
					"startTime":                      parseTime("2018-02-14T16:26:33Z").Unix(),
					"createdKind":                    "DaemonSet",
					"createdBy":                      "newrelic-infra",
					"nodeIP":                         "192.168.99.100",
					"namespace":                      "kube-system",
					"namespaceName":                  "kube-system",
					"nodeName":                       "minikube",
					"podName":                        "newrelic-infra-rz225",
					"isReady":                        1,
					"status":                         "Running",
					"isScheduled":                    1,
					"label.controller-revision-hash": "3887482659",
					"label.name":                     "newrelic-infra",
					"label.pod-template-generation":  "1",
					"displayName":                    "newrelic-infra-rz225", // From manipulator
					"clusterName":                    "test-cluster",         // From manipulator
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

					"entityName":            "k8s:test-cluster:kube-system:newrelic-infra-rz225:container:newrelic-infra",
					"event_type":            "K8sContainerSample",
					"memoryUsedBytes":       uint64(18083840),
					"memoryWorkingSetBytes": uint64(17113088),
					"cpuUsedCores":          0.01742824,
					"fsAvailableBytes":      uint64(14924988416),
					"fsUsedBytes":           uint64(126976),
					"fsUsedPercent":         float64(0.0008507538914443524),
					"fsCapacityBytes":       uint64(17293533184),
					"fsInodesFree":          uint64(9713372),
					"fsInodes":              uint64(9732096),
					"fsInodesUsed":          uint64(36),
					"containerName":         "newrelic-infra",
					"containerID":           "69d7203a8f2d2d027ffa51d61002eac63357f22a17403363ef79e66d1c3146b2",
					"containerImage":        "newrelic/ohaik:1.0.0-beta3",
					"containerImageID":      "sha256:1a95d0df2997f93741fbe2a15d2c31a394e752fd942ec29bf16a44163342f6a1",
					"namespace":             "kube-system",
					"namespaceName":         "kube-system",
					"podName":               "newrelic-infra-rz225",
					"nodeName":              "minikube",
					"nodeIP":                "192.168.99.100",
					"restartCount":          int32(6),
					"cpuRequestedCores":     0.1,
					"memoryRequestedBytes":  int64(104857600),
					"memoryLimitBytes":      int64(104857600),
					"status":                "Running",
					"isReady":               1,
					//"reason":               "",      // TODO ?
					"displayName":                    "newrelic-infra", // From manipulator
					"clusterName":                    "test-cluster",   // From manipulator
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

	// Expected errs (missing data)
	// TODO not good to compare error strings...
	expectedErrs := []error{
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric deploymentName, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric reason, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric message, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric deploymentName, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric cpuLimitCores, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric reason, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric cpuCoresUtilization, 'cpuUsedCores' is nil"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric requestedCpuCoresUtilization, 'cpuUsedCores' is nil"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric memoryUtilization, 'memoryUsedBytes' is nil"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric requestedMemoryUtilization, 'memoryUsedBytes' is nil"),
	}

	assert.ElementsMatch(t, expectedErrs, err.(data.PopulateResult).Errors)
	expectedInventory := inventory.New()
	expectedInventory.SetItem("cluster", "name", expectedEntities[0].Metadata.Name)
	expectedInventory.SetItem("cluster", "k8sVersion", k8sVersion.String())
	expectedEntities[0].Inventory = expectedInventory

	if len(expectedEntities) != len(intgr.Entities) {
		t.Fatalf("missing required entities")
	}

	// Sort slices, so we can later diff them one-by-one for decent readability
	entitySliceLesser := func(entities []*integration.Entity) func(i, j int) bool {
		return func(i, j int) bool {
			return strings.Compare(entities[i].Metadata.Name, entities[j].Metadata.Name) < 0
		}
	}

	sort.Slice(intgr.Entities, entitySliceLesser(intgr.Entities))
	sort.Slice(expectedEntities, entitySliceLesser(expectedEntities))

	// Compare entities deeply, one by one, ignoring unexported fields
	for j := range expectedEntities {
		compareIgnoreFields := cmpopts.IgnoreUnexported(integration.Entity{}, metric.Set{}, inventory.Inventory{})
		e := intgr.Entities[j]
		ee := expectedEntities[j]
		if !cmp.Equal(e, ee, compareIgnoreFields) {
			t.Fatalf("Entities[%d] mismatch: %s", j, cmp.Diff(e, ee, compareIgnoreFields))
		}
	}
}
