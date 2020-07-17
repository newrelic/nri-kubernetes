package metric

import (
	"errors"
	"testing"

	"time"

	sdkMetric "github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/kubelet/metric/testdata"
	"github.com/stretchr/testify/assert"
)

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339, raw)

	return t
}

var expectedMetrics = []*sdk.EntityData{
	{
		Entity: sdk.Entity{
			Name: "test-cluster",
			Type: "k8s:cluster",
		},
		Metrics: []sdkMetric.MetricSet{
			{
				"entityName":  "k8s:cluster:test-cluster",
				"event_type":  "K8sClusterSample",
				"clusterName": "test-cluster",
			},
		},
		Inventory: sdk.Inventory{},
		Events:    []*sdk.Event{},
	},
	{
		Entity: sdk.Entity{
			Name: "newrelic-infra-rz225",
			Type: "k8s:test-cluster:kube-system:pod",
		},
		Metrics: []sdkMetric.MetricSet{
			{
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
		Inventory: sdk.Inventory{},
		Events:    []*sdk.Event{},
	},
	{
		Entity: sdk.Entity{
			Name: "newrelic-infra",
			Type: "k8s:test-cluster:kube-system:newrelic-infra-rz225:container",
		},
		Metrics: []sdkMetric.MetricSet{
			{
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
		Inventory: sdk.Inventory{},
		Events:    []*sdk.Event{},
	},
}

// We reduce the test fixtures in order to simplify testing.
var kubeletSpecs = definition.SpecGroups{
	"pod":       KubeletSpecs["pod"],
	"container": KubeletSpecs["container"],
}

func TestPopulateK8s(t *testing.T) {
	p := NewK8sPopulator()

	i, err := sdk.NewIntegrationProtocol2("test", "test", new(struct{}))
	assert.NoError(t, err)
	i.Clear()

	// We reduce the test fixtures in order to simplify testing.
	foo := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-rz225": testdata.ExpectedGroupData["pod"]["kube-system_newrelic-infra-rz225"],
		},
		"container": {
			"kube-system_newrelic-infra-rz225_newrelic-infra": testdata.ExpectedGroupData["container"]["kube-system_newrelic-infra-rz225_newrelic-infra"],
		},
	}
	err = p.Populate(foo, kubeletSpecs, i, "test-cluster")
	assert.Error(t, err)

	// Expected errs (missing data)
	expectedErrs := []error{
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric deploymentName, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric reason, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225: cannot fetch value for metric message, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric deploymentName, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric cpuLimitCores, metric not found"),
		errors.New("error populating metric for entity ID kube-system_newrelic-infra-rz225_newrelic-infra: cannot fetch value for metric reason, metric not found"),
	}

	assert.ElementsMatch(t, expectedErrs, err.(*data.PopulateErr).Errs)
	expectedInventory := sdk.Inventory{}
	expectedInventory.SetItem("cluster", "name", expectedMetrics[0].Entity.Name)
	expectedMetrics[0].Inventory = expectedInventory
	assert.ElementsMatch(t, expectedMetrics, i.Data)
}
