package metric

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	v1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var responseContainerWithTheSameName = `{ "pods": [ { "podRef": { "name": "newrelic-infra-monitoring-pjp0v", "namespace": "kube-system", "uid": "b5a9c98f-d34f-11e7-95fe-62d16fb0cc7f" }, "startTime": "2017-11-30T09:12:37Z", "containers": [ { "name": "kube-state-metrics", "startTime": "2017-11-30T09:12:51Z", "cpu": { "time": "2017-11-30T14:48:10Z", "usageNanoCores": 184087, "usageCoreNanoSeconds": 4284675040 }, "memory": { "time": "2017-11-30T14:48:10Z", "usageBytes": 22552576, "workingSetBytes": 15196160, "rssBytes": 7352320, "pageFaults": 4683, "majorPageFaults": 152 }, "rootfs": { "time": "2017-11-30T14:48:10Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 35000320, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 24 }, "logs": { "time": "2017-11-30T14:48:10Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 20480, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 157225 }, "userDefinedMetrics": null }, { "name": "newrelic-infra", "startTime": "2017-11-30T09:12:44Z", "cpu": { "time": "2017-11-30T14:48:12Z", "usageNanoCores": 13046199, "usageCoreNanoSeconds": 303855795298 }, "memory": { "time": "2017-11-30T14:48:12Z", "usageBytes": 243638272, "workingSetBytes": 38313984, "rssBytes": 15785984, "pageFaults": 10304448, "majorPageFaults": 217 }, "rootfs": { "time": "2017-11-30T14:48:12Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 1305837568, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 52 }, "logs": { "time": "2017-11-30T14:48:12Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 657747968, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 157225 }, "userDefinedMetrics": null } ], "network": { "time": "2017-11-30T14:48:12Z", "rxBytes": 15741653, "rxErrors": 0, "txBytes": 19551073, "txErrors": 0 }, "volume": [ { "time": "2017-11-30T09:13:29Z", "availableBytes": 1048637440, "capacityBytes": 1048649728, "usedBytes": 12288, "inodesFree": 256009, "inodes": 256018, "inodesUsed": 9, "name": "default-token-7cg8m" } ] }, { "podRef": { "name": "kube-dns-910330662-pflkj", "namespace": "kube-system", "uid": "a6f2130b-a21e-11e7-8db6-62d16fb0cc7f" }, "startTime": "2017-11-30T09:12:36Z", "containers": [ { "name": "kube-state-metrics", "startTime": "2017-11-30T09:12:51Z", "cpu": { "time": "2017-11-30T14:48:10Z", "usageNanoCores": 184087, "usageCoreNanoSeconds": 4284675040 }, "memory": { "time": "2017-11-30T14:48:10Z", "usageBytes": 22552576, "workingSetBytes": 15196160, "rssBytes": 7352320, "pageFaults": 4683, "majorPageFaults": 152 }, "rootfs": { "time": "2017-11-30T14:48:10Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 35000320, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 24 }, "logs": { "time": "2017-11-30T14:48:10Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 20480, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 157225 }, "userDefinedMetrics": null }, { "name": "dnsmasq", "startTime": "2017-11-30T09:12:43Z", "cpu": { "time": "2017-11-30T14:48:07Z", "usageNanoCores": 208374, "usageCoreNanoSeconds": 3653471654 }, "memory": { "time": "2017-11-30T14:48:07Z", "usageBytes": 19812352, "workingSetBytes": 12828672, "rssBytes": 5201920, "pageFaults": 3376, "majorPageFaults": 139 }, "rootfs": { "time": "2017-11-30T14:48:07Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 42041344, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 20 }, "logs": { "time": "2017-11-30T14:48:07Z", "availableBytes": 6911750144, "capacityBytes": 17293533184, "usedBytes": 20480, "inodesFree": 9574871, "inodes": 9732096, "inodesUsed": 157225 }, "userDefinedMetrics": null } ], "network": { "time": "2017-11-30T14:48:07Z", "rxBytes": 14447980, "rxErrors": 0, "txBytes": 15557657, "txErrors": 0 }, "volume": [ { "time": "2017-11-30T09:13:29Z", "availableBytes": 1048637440, "capacityBytes": 1048649728, "usedBytes": 12288, "inodesFree": 256009, "inodes": 256018, "inodesUsed": 9, "name": "default-token-7cg8m" } ] } ] }`
var responseMissingContainerName = `{ "pods": [ { "podRef": { "name": "newrelic-infra-monitoring-pjp0v", "namespace": "kube-system", "uid": "b5a9c98f-d34f-11e7-95fe-62d16fb0cc7f" }, "startTime": "2017-11-30T09:12:37Z", "containers": [ { "startTime": "2017-11-30T09:12:51Z", "cpu": { "time": "2017-11-30T14:48:10Z", "usageNanoCores": 184087, "usageCoreNanoSeconds": 4284675040 }, "memory": { "time": "2017-11-30T14:48:10Z", "usageBytes": 22552576, "workingSetBytes": 15196160, "rssBytes": 7352320, "pageFaults": 4683, "majorPageFaults": 152 } } ], "network": { "time": "2017-11-30T14:48:12Z", "rxBytes": 15741653, "txBytes": 52463212, "rxErrors": 0,  "txErrors": 0 } } ] }`
var responseMissingPodName = `{ "pods": [ { "podRef": { "namespace": "kube-system", "uid": "b5a9c98f-d34f-11e7-95fe-62d16fb0cc7f" }, "startTime": "2017-11-30T09:12:37Z", "containers": [ { "name": "kube-state-metrics", "startTime": "2017-11-30T09:12:51Z", "cpu": { "time": "2017-11-30T14:48:10Z", "usageNanoCores": 184087, "usageCoreNanoSeconds": 4284675040 }, "memory": { "time": "2017-11-30T14:48:10Z", "usageBytes": 22552576, "workingSetBytes": 15196160, "rssBytes": 7352320, "pageFaults": 4683, "majorPageFaults": 152 } } ], "network": { "time": "2017-11-30T14:48:12Z", "rxBytes": 15741653, "txBytes": 52463212, "rxErrors": 0,  "txErrors": 0 } } ] }`
var responseMissingRxBytesForPod = `{ "pods": [ { "podRef": { "name": "newrelic-infra-monitoring-pjp0v", "namespace": "kube-system", "uid": "b5a9c98f-d34f-11e7-95fe-62d16fb0cc7f" }, "startTime": "2017-11-30T09:12:37Z", "containers": [ { "name": "kube-state-metrics", "startTime": "2017-11-30T09:12:51Z", "cpu": { "time": "2017-11-30T14:48:10Z", "usageNanoCores": 184087, "usageCoreNanoSeconds": 4284675040 }, "memory": { "time": "2017-11-30T14:48:10Z", "usageBytes": 22552576, "workingSetBytes": 15196160, "rssBytes": 7352320, "pageFaults": 4683, "majorPageFaults": 152 } } ], "network": { "time": "2017-11-30T14:48:12Z", "txBytes": 52463212, "rxErrors": 0,  "txErrors": 0 } } ] }`

var nodeSampleMissingImageFs = `{ "node": { "nodeName": "fooNode", "startTime": "2018-01-22T06:52:15Z", "cpu": { "time": "2018-01-24T16:40:00Z", "usageNanoCores": 64124211, "usageCoreNanoSeconds": 353998913059080 }, "memory": { "time": "2018-01-24T16:40:00Z", "availableBytes": 502603776, "usageBytes": 687067136, "workingSetBytes": 540618752, "rssBytes": 150396928, "pageFaults": 3067606235, "majorPageFaults": 517653 }, "network": { "time": "2018-01-24T16:40:00Z", "rxBytes": 51419684038, "rxErrors": 0, "txBytes": 25630208577, "txErrors": 0, "interfaces": [ { "name": "ens5", "rxBytes": 51419684038, "rxErrors": 0, "txBytes": 25630208577, "txErrors": 0 }, { "name": "ip6tnl0", "rxBytes": 0, "rxErrors": 0, "txBytes": 0, "txErrors": 0 } ] }, "fs": { "time": "2018-01-24T16:40:00Z", "availableBytes": 92795400192, "capacityBytes": 128701009920, "usedBytes": 30305800192, "inodesFree": 32999604, "inodes": 33554432, "inodesUsed": 554828 }, "runtime": { } } }`

func toSummary(response string) (v1.Summary, error) {
	var summary = new(v1.Summary)
	err := json.Unmarshal([]byte(response), summary)
	if err != nil {
		return v1.Summary{}, fmt.Errorf("Error unmarshaling the response body. Got error: %v", err.Error())
	}
	return *summary, nil
}

func TestGroupStatsSummary_CorrectValue(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"nodeName": "fooNode",
				// CPU
				"usageNanoCores":       uint64(64124211),
				"usageCoreNanoSeconds": uint64(353998913059080),
				// Memory
				"memoryUsageBytes":      uint64(687067136),
				"memoryAvailableBytes":  uint64(502603776),
				"memoryWorkingSetBytes": uint64(540618752),
				"memoryRssBytes":        uint64(150396928),
				"memoryPageFaults":      uint64(3067606235),
				"memoryMajorPageFaults": uint64(517653),
				// Network
				"rxBytes": uint64(51419684038),
				"txBytes": uint64(25630208577),
				"errors":  uint64(0),
				"interfaces": map[string]definition.RawMetrics{
					"ens5": {
						"rxBytes": uint64(51419684038),
						"txBytes": uint64(25630208577),
						"errors":  uint64(0),
					},
					"ip6tnl0": {
						"rxBytes": uint64(0),
						"txBytes": uint64(0),
						"errors":  uint64(0),
					},
				},
				// Fs
				"fsAvailableBytes": uint64(92795400192),
				"fsCapacityBytes":  uint64(128701009920),
				"fsUsedBytes":      uint64(30305800192),
				"fsInodesFree":     uint64(32999604),
				"fsInodes":         uint64(33554432),
				"fsInodesUsed":     uint64(554828),
				// Runtime
				"runtimeAvailableBytes": uint64(92795400192),
				"runtimeCapacityBytes":  uint64(128701009920),
				"runtimeUsedBytes":      uint64(20975835934),
				"runtimeInodesFree":     uint64(32999604),
				"runtimeInodes":         uint64(33554432),
				"runtimeInodesUsed":     uint64(554828),
			},
		},
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "kube-system",
				"rxBytes":   uint64(15741653),
				"errors":    uint64(0),
				"txBytes":   uint64(19551073),
				"interfaces": map[string]definition.RawMetrics{
					"eth0": {
						"rxBytes": uint64(15741653),
						"errors":  uint64(0),
						"txBytes": uint64(19551073),
					},
				},
			},
			"kube-system_kube-dns-910330662-pflkj": definition.RawMetrics{
				"podName":   "kube-dns-910330662-pflkj",
				"namespace": "kube-system",
				"rxBytes":   uint64(14447980),
				"errors":    uint64(0),
				"txBytes":   uint64(15557657),
				"interfaces": map[string]definition.RawMetrics{
					"eth0": {
						"rxBytes": uint64(14447980),
						"errors":  uint64(0),
						"txBytes": uint64(15557657),
					},
				},
			},
		},
		"volume": {
			"kube-system_kube-dns-910330662-pflkj_default-token-7cg8m": definition.RawMetrics{
				"fsAvailableBytes": uint64(1048637440),
				"fsCapacityBytes":  uint64(1048649728),
				"fsInodes":         uint64(256018),
				"fsInodesFree":     uint64(256009),
				"fsInodesUsed":     uint64(9),
				"namespace":        "kube-system",
				"podName":          "kube-dns-910330662-pflkj",
				"fsUsedBytes":      uint64(12288),
				"volumeName":       "default-token-7cg8m",
			},
			"kube-system_newrelic-infra-monitoring-pjp0v_default-token-7cg8m": definition.RawMetrics{
				"fsAvailableBytes": uint64(1048637440),
				"fsCapacityBytes":  uint64(1048649728),
				"fsInodes":         uint64(256018),
				"fsInodesFree":     uint64(256009),
				"fsInodesUsed":     uint64(9),
				"namespace":        "kube-system",
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"fsUsedBytes":      uint64(12288),
				"volumeName":       "default-token-7cg8m",
			},
		},
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName":    "kube-state-metrics",
				"usageBytes":       uint64(22552576),
				"workingSetBytes":  uint64(15196160),
				"usageNanoCores":   uint64(184087),
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(24),
				"fsUsedBytes":      uint64(35000320),
			},
			"kube-system_newrelic-infra-monitoring-pjp0v_newrelic-infra": definition.RawMetrics{
				"containerName":    "newrelic-infra",
				"usageBytes":       uint64(243638272),
				"workingSetBytes":  uint64(38313984),
				"usageNanoCores":   uint64(13046199),
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(52),
				"fsUsedBytes":      uint64(1305837568),
			},
			"kube-system_kube-dns-910330662-pflkj_dnsmasq": definition.RawMetrics{
				"containerName":    "dnsmasq",
				"usageBytes":       uint64(19812352),
				"workingSetBytes":  uint64(12828672),
				"usageNanoCores":   uint64(208374),
				"podName":          "kube-dns-910330662-pflkj",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(20),
				"fsUsedBytes":      uint64(42041344),
			},
		},
	}

	responseOkDataSample, err := ioutil.ReadFile("testdata/kubelet_summary_response_ok.json")
	require.NoError(t, err)
	summary, err := toSummary(string(responseOkDataSample))
	assert.NoError(t, err)

	rawData, errs := GroupStatsSummary(summary)
	assert.Empty(t, errs)
	assert.Equal(t, expectedRawData, rawData)
}

func TestGroupStatsSummary_MissingNodeData_ContainerWithTheSameName(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":    "newrelic-infra-monitoring-pjp0v",
				"namespace":  "kube-system",
				"rxBytes":    uint64(15741653),
				"errors":     uint64(0),
				"txBytes":    uint64(19551073),
				"interfaces": make(map[string]definition.RawMetrics),
			},
			"kube-system_kube-dns-910330662-pflkj": definition.RawMetrics{
				"podName":    "kube-dns-910330662-pflkj",
				"namespace":  "kube-system",
				"rxBytes":    uint64(14447980),
				"errors":     uint64(0),
				"txBytes":    uint64(15557657),
				"interfaces": make(map[string]definition.RawMetrics),
			},
		},
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName":    "kube-state-metrics",
				"usageBytes":       uint64(22552576),
				"workingSetBytes":  uint64(15196160),
				"usageNanoCores":   uint64(184087),
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(24),
				"fsUsedBytes":      uint64(35000320),
			},
			"kube-system_kube-dns-910330662-pflkj_kube-state-metrics": definition.RawMetrics{
				"containerName":    "kube-state-metrics",
				"usageBytes":       uint64(22552576),
				"workingSetBytes":  uint64(15196160),
				"usageNanoCores":   uint64(184087),
				"podName":          "kube-dns-910330662-pflkj",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(24),
				"fsUsedBytes":      uint64(35000320),
			},
			"kube-system_newrelic-infra-monitoring-pjp0v_newrelic-infra": definition.RawMetrics{
				"containerName":    "newrelic-infra",
				"usageBytes":       uint64(243638272),
				"workingSetBytes":  uint64(38313984),
				"usageNanoCores":   uint64(13046199),
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(52),
				"fsUsedBytes":      uint64(1305837568),
			},
			"kube-system_kube-dns-910330662-pflkj_dnsmasq": definition.RawMetrics{
				"containerName":    "dnsmasq",
				"usageBytes":       uint64(19812352),
				"workingSetBytes":  uint64(12828672),
				"usageNanoCores":   uint64(208374),
				"podName":          "kube-dns-910330662-pflkj",
				"namespace":        "kube-system",
				"fsAvailableBytes": uint64(6911750144),
				"fsCapacityBytes":  uint64(17293533184),
				"fsInodes":         uint64(9732096),
				"fsInodesFree":     uint64(9574871),
				"fsInodesUsed":     uint64(20),
				"fsUsedBytes":      uint64(42041344),
			},
		},
		"volume": {
			"kube-system_kube-dns-910330662-pflkj_default-token-7cg8m": definition.RawMetrics{
				"fsAvailableBytes": uint64(1048637440),
				"fsCapacityBytes":  uint64(1048649728),
				"fsInodes":         uint64(256018),
				"fsInodesFree":     uint64(256009),
				"fsInodesUsed":     uint64(9),
				"namespace":        "kube-system",
				"podName":          "kube-dns-910330662-pflkj",
				"fsUsedBytes":      uint64(12288),
				"volumeName":       "default-token-7cg8m",
			},
			"kube-system_newrelic-infra-monitoring-pjp0v_default-token-7cg8m": definition.RawMetrics{
				"fsAvailableBytes": uint64(1048637440),
				"fsCapacityBytes":  uint64(1048649728),
				"fsInodes":         uint64(256018),
				"fsInodesFree":     uint64(256009),
				"fsInodesUsed":     uint64(9),
				"namespace":        "kube-system",
				"podName":          "newrelic-infra-monitoring-pjp0v",
				"fsUsedBytes":      uint64(12288),
				"volumeName":       "default-token-7cg8m",
			},
		},
		"node": {},
	}
	summary, err := toSummary(responseContainerWithTheSameName)
	assert.NoError(t, err)

	rawData, errs := GroupStatsSummary(summary)
	assert.EqualError(t, errs[0], "empty node identifier, possible data error in /stats/summary response")
	assert.Equal(t, expectedRawData, rawData)
}

func TestGroupStatsSummary_IncompleteStatsSummaryMessage_MissingNodeData_MissingContainerName(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":    "newrelic-infra-monitoring-pjp0v",
				"namespace":  "kube-system",
				"rxBytes":    uint64(15741653),
				"txBytes":    uint64(52463212),
				"errors":     uint64(0),
				"interfaces": make(map[string]definition.RawMetrics),
			},
		},
		"volume":    {},
		"container": {},
		"node":      {},
	}

	summary, err := toSummary(responseMissingContainerName)
	assert.NoError(t, err)

	rawData, errs := GroupStatsSummary(summary)
	assert.Len(t, errs, 2, "Not expected length of errors")
	assert.Equal(t, expectedRawData, rawData)
}

func TestGroupStatsSummary_IncompleteStatsSummaryMessage_MissingNodeData_MissingPodName(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"pod":       {},
		"container": {},
		"volume":    {},
		"node":      {},
	}

	summary, err := toSummary(responseMissingPodName)
	assert.NoError(t, err)

	rawData, errs := GroupStatsSummary(summary)
	assert.Len(t, errs, 2, "Not expected length of errors")
	assert.Len(t, rawData, 4, "Not expected length of rawData for pods and containers")
	assert.Equal(t, expectedRawData, rawData)
	assert.Empty(t, rawData["pod"])
	assert.Empty(t, rawData["container"])
	assert.Empty(t, rawData["node"])
	assert.Empty(t, rawData["volume"])
}

func TestGroupStatsSummary_IncompleteStatsSummaryMessage_MissingNodeData_NoRxBytesForPod_ReportedAsZero(t *testing.T) {
	expectedRawData := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":    "newrelic-infra-monitoring-pjp0v",
				"namespace":  "kube-system",
				"errors":     uint64(0),
				"txBytes":    uint64(52463212),
				"interfaces": make(map[string]definition.RawMetrics),
			},
		},
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName":   "kube-state-metrics",
				"usageBytes":      uint64(22552576),
				"workingSetBytes": uint64(15196160),
				"usageNanoCores":  uint64(184087),
				"podName":         "newrelic-infra-monitoring-pjp0v",
				"namespace":       "kube-system",
			},
		},
		"volume": {},
		"node":   {},
	}

	summary, err := toSummary(responseMissingRxBytesForPod)
	assert.NoError(t, err)

	rawData, errs := GroupStatsSummary(summary)
	assert.Len(t, errs, 1, "Not expected length of errors")
	assert.Equal(t, expectedRawData, rawData)
}

func TestGroupStatsSummary_EmptyStatsSummaryMessage(t *testing.T) {
	var summary = new(v1.Summary)

	rawData, errs := GroupStatsSummary(*summary)

	assert.Len(t, errs, 2, "Not expected length of errors")
	assert.Len(t, rawData, 4, "Not expected length of rawData for pods and containers")
	assert.Empty(t, rawData["pod"])
	assert.Empty(t, rawData["container"])
	assert.Empty(t, rawData["node"])
	assert.Empty(t, rawData["volume"])
}

func TestAddUint64RawMetric(t *testing.T) {
	r := definition.RawMetrics{
		"nodeName": "fooNode",
	}

	expected := definition.RawMetrics{
		"nodeName": "fooNode",
		"foo":      uint64(353998913059080),
	}

	summary, err := toSummary(`{ "node": { "cpu": { "usageCoreNanoSeconds": 353998913059080 } } }`)
	assert.NoError(t, err)

	AddUint64RawMetric(r, "foo", summary.Node.CPU.UsageCoreNanoSeconds)
	assert.Equal(t, expected, r)
}

func TestFetchNodeStats_MissingImageFs(t *testing.T) {
	expectedRawData := definition.RawMetrics{
		"nodeName": "fooNode",
		// CPU
		"usageNanoCores":       uint64(64124211),
		"usageCoreNanoSeconds": uint64(353998913059080),
		// Memory
		"memoryUsageBytes":      uint64(687067136),
		"memoryAvailableBytes":  uint64(502603776),
		"memoryWorkingSetBytes": uint64(540618752),
		"memoryRssBytes":        uint64(150396928),
		"memoryPageFaults":      uint64(3067606235),
		"memoryMajorPageFaults": uint64(517653),
		// Network
		"rxBytes": uint64(51419684038),
		"txBytes": uint64(25630208577),
		"errors":  uint64(0),
		"interfaces": map[string]definition.RawMetrics{
			"ens5": {
				"rxBytes": uint64(51419684038),
				"txBytes": uint64(25630208577),
				"errors":  uint64(0),
			},
			"ip6tnl0": {
				"rxBytes": uint64(0),
				"txBytes": uint64(0),
				"errors":  uint64(0),
			},
		},
		// Fs
		"fsAvailableBytes": uint64(92795400192),
		"fsCapacityBytes":  uint64(128701009920),
		"fsUsedBytes":      uint64(30305800192),
		"fsInodesFree":     uint64(32999604),
		"fsInodes":         uint64(33554432),
		"fsInodesUsed":     uint64(554828),
	}
	s, err := toSummary(nodeSampleMissingImageFs)
	assert.NoError(t, err)

	rawData, ID, errs := fetchNodeStats(s.Node)
	assert.Empty(t, errs)
	assert.Equal(t, "fooNode", ID)
	assert.Equal(t, expectedRawData, rawData)
}

// ------------ FromRawGroupsEntityIDGenerator ------------
func TestFromRawGroupsEntityIDGenerator_CorrectValue(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName": "kube-state-metrics",
				"podName":       "newrelic-infra-monitoring-pjp0v",
				"namespace":     "kube-system",
			},
		},
	}
	expectedValue := "newrelic-infra-monitoring-pjp0v"

	generatedValue, err := FromRawGroupsEntityIDGenerator("podName")("container", "kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics", raw)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromRawGroupsEntityIDGenerator_NotFound(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName": "kube-state-metrics",
				"namespace":     "kube-system",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityIDGenerator("podName")("container", "kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics", raw)
	assert.EqualError(t, err, "\"podName\" not found for \"container\"")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityIDGenerator_IncorrectType(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName": "kube-state-metrics",
				"podName":       1,
				"namespace":     "kube-system",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityIDGenerator("podName")("container", "kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics", raw)
	assert.EqualError(t, err, "incorrect type of \"podName\" for \"container\"")
	assert.Equal(t, "", generatedValue)
}

// ------------ FromRawEntityIDGroupEntityIDGenerator ------------
func TestFromRawEntityIDGroupEntityIDGenerator_CorrectValue(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "kube-system",
			},
		},
	}
	expectedValue := "newrelic-infra-monitoring-pjp0v"

	generatedValue, err := FromRawEntityIDGroupEntityIDGenerator("namespace")("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw)
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromRawEntityIDGroupEntityIDGenerator_NotFound(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName": "newrelic-infra-monitoring-pjp0v",
			},
		},
	}
	expectedValue := ""

	generatedValue, err := FromRawEntityIDGroupEntityIDGenerator("namespace")("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw)
	assert.EqualError(t, err, "\"namespace\" not found for \"pod\"")
	assert.Equal(t, expectedValue, generatedValue)
}

// ------------ FromRawGroupsEntityTypeGenerator -----------------
func TestFromRawGroupsEntityTypeGenerator_CorrectValueNode(t *testing.T) {
	raw := definition.RawGroups{
		"node": {
			"fooNode": definition.RawMetrics{
				"nodeName": "fooNode",
			},
		},
	}
	expectedValue := "k8s:clusterName:node"

	generatedValue, err := FromRawGroupsEntityTypeGenerator("node", "fooNode", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_CorrectValueContainer(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName": "kube-state-metrics",
				"podName":       "newrelic-infra-monitoring-pjp0v",
				"namespace":     "kube-system",
			},
		},
	}
	expectedValue := "k8s:clusterName:kube-system:newrelic-infra-monitoring-pjp0v:container"

	generatedValue, err := FromRawGroupsEntityTypeGenerator("container", "kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_CorrectValuePod(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "kube-system",
			},
		},
	}
	expectedValue := "k8s:clusterName:kube-system:pod"

	generatedValue, err := FromRawGroupsEntityTypeGenerator("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw, "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, expectedValue, generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_GroupLabelNotFound(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "kube-system",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("foo", "kube-system_newrelic-infra-monitoring-pjp0v", raw, "clusterName")
	assert.EqualError(t, err, "\"foo\" not found")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_RawEntityIDNotFound(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "kube-system",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("pod", "foo", raw, "clusterName")
	assert.EqualError(t, err, "entity data \"foo\" not found for \"pod\"")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_KeyNotFound(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName": "newrelic-infra-monitoring-pjp0v",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw, "clusterName")
	assert.EqualError(t, err, "\"namespace\" not found for \"pod\"")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_IncorrectType(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": 1,
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw, "clusterName")
	assert.EqualError(t, err, "incorrect type of \"namespace\" for \"pod\"")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_EmptyNamespace(t *testing.T) {
	raw := definition.RawGroups{
		"pod": {
			"kube-system_newrelic-infra-monitoring-pjp0v": definition.RawMetrics{
				"podName":   "newrelic-infra-monitoring-pjp0v",
				"namespace": "",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("pod", "kube-system_newrelic-infra-monitoring-pjp0v", raw, "clusterName")
	assert.EqualError(t, err, "empty namespace for generated entity type for \"pod\"")
	assert.Equal(t, "", generatedValue)
}

func TestFromRawGroupsEntityTypeGenerator_EmptyPodName(t *testing.T) {
	raw := definition.RawGroups{
		"container": {
			"kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics": definition.RawMetrics{
				"containerName": "kube-state-metrics",
				"podName":       "",
				"namespace":     "kube-system",
			},
		},
	}

	generatedValue, err := FromRawGroupsEntityTypeGenerator("container", "kube-system_newrelic-infra-monitoring-pjp0v_kube-state-metrics", raw, "clusterName")
	assert.EqualError(t, err, "empty values for generated entity type for \"container\"")
	assert.Equal(t, "", generatedValue)
}
