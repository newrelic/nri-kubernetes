package metric

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// StatsSummaryPath is the path where kubelet serves a summary with several information.
const StatsSummaryPath = "/stats/summary"

// GetMetricsData calls kubelet /stats/summary endpoint and returns unmarshalled response
func GetMetricsData(c client.HTTPGetter) (*v1.Summary, error) {
	resp, err := c.Get(StatsSummaryPath)
	if err != nil {
		return nil, fmt.Errorf("performing GET request to kubelet endpoint %q: %w", StatsSummaryPath, err)
	}
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		// Cap error body at 1MB — this is only used for logging, so we don't need the full response.
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

		bodyErr := fmt.Errorf("response body: %s", string(body))

		if err != nil {
			bodyErr = fmt.Errorf("reading response body: %w", err)
		}

		return nil, fmt.Errorf("received non-OK response code from kubelet: %d: %w", resp.StatusCode, bodyErr)
	}

	summary := &v1.Summary{}

	if err := json.NewDecoder(resp.Body).Decode(summary); err != nil {
		return nil, fmt.Errorf("unmarshaling the response body into kubelet stats Summary: %w", err)
	}

	return summary, nil
}

func fetchNodeStats(n v1.NodeStats) (definition.RawMetrics, string, error) {
	r := make(definition.RawMetrics)

	nodeName := n.NodeName
	if nodeName == "" {
		return r, "", fmt.Errorf("empty node identifier, possible data error in %s response", StatsSummaryPath)
	}

	r["nodeName"] = nodeName

	if n.CPU != nil {
		AddUint64RawMetric(r, "usageNanoCores", n.CPU.UsageNanoCores)
		AddUint64RawMetric(r, "usageCoreNanoSeconds", n.CPU.UsageCoreNanoSeconds)
	}

	if n.Memory != nil {
		AddUint64RawMetric(r, "memoryUsageBytes", n.Memory.UsageBytes)
		AddUint64RawMetric(r, "memoryAvailableBytes", n.Memory.AvailableBytes)
		AddUint64RawMetric(r, "memoryWorkingSetBytes", n.Memory.WorkingSetBytes)
		AddUint64RawMetric(r, "memoryRssBytes", n.Memory.RSSBytes)
		AddUint64RawMetric(r, "memoryPageFaults", n.Memory.PageFaults)
		AddUint64RawMetric(r, "memoryMajorPageFaults", n.Memory.MajorPageFaults)
	}

	if n.Network != nil {
		AddUint64RawMetric(r, "rxBytes", n.Network.RxBytes)
		AddUint64RawMetric(r, "txBytes", n.Network.TxBytes)
		if n.Network.RxErrors != nil && n.Network.TxErrors != nil {
			r["errors"] = *n.Network.RxErrors + *n.Network.TxErrors
		}

		interfaces := make(map[string]definition.RawMetrics)
		for _, i := range n.Network.Interfaces {
			interfaceMetrics := make(definition.RawMetrics)
			AddUint64RawMetric(interfaceMetrics, "rxBytes", i.RxBytes)
			AddUint64RawMetric(interfaceMetrics, "txBytes", i.TxBytes)
			if i.RxErrors != nil && i.TxErrors != nil {
				interfaceMetrics["errors"] = *i.RxErrors + *i.TxErrors
			}
			interfaces[i.Name] = interfaceMetrics
		}
		r["interfaces"] = interfaces
	}

	if n.Fs != nil {
		AddUint64RawMetric(r, "fsAvailableBytes", n.Fs.AvailableBytes)
		AddUint64RawMetric(r, "fsCapacityBytes", n.Fs.CapacityBytes)
		AddUint64RawMetric(r, "fsUsedBytes", n.Fs.UsedBytes)
		AddUint64RawMetric(r, "fsInodesFree", n.Fs.InodesFree)
		AddUint64RawMetric(r, "fsInodes", n.Fs.Inodes)
		AddUint64RawMetric(r, "fsInodesUsed", n.Fs.InodesUsed)
	}
	if n.Runtime != nil && n.Runtime.ImageFs != nil {
		AddUint64RawMetric(r, "runtimeAvailableBytes", n.Runtime.ImageFs.AvailableBytes)
		AddUint64RawMetric(r, "runtimeCapacityBytes", n.Runtime.ImageFs.CapacityBytes)
		AddUint64RawMetric(r, "runtimeUsedBytes", n.Runtime.ImageFs.UsedBytes)
		AddUint64RawMetric(r, "runtimeInodesFree", n.Runtime.ImageFs.InodesFree)
		AddUint64RawMetric(r, "runtimeInodes", n.Runtime.ImageFs.Inodes)
		AddUint64RawMetric(r, "runtimeInodesUsed", n.Runtime.ImageFs.InodesUsed)
	}

	return r, nodeName, nil
}

func fetchPodStats(pod v1.PodStats) (definition.RawMetrics, string, error) {
	r := make(definition.RawMetrics)

	if pod.PodRef.Name == "" || pod.PodRef.Namespace == "" {
		return r, "", fmt.Errorf("empty pod identifier, possible data error in %s response", StatsSummaryPath)
	}

	r["podName"] = pod.PodRef.Name
	r["namespace"] = pod.PodRef.Namespace

	if pod.Network != nil {
		AddUint64RawMetric(r, "rxBytes", pod.Network.RxBytes)
		AddUint64RawMetric(r, "txBytes", pod.Network.TxBytes)
		if pod.Network.RxErrors != nil && pod.Network.TxErrors != nil {
			r["errors"] = *pod.Network.RxErrors + *pod.Network.TxErrors
		}
		interfaces := make(map[string]definition.RawMetrics)
		for _, i := range pod.Network.Interfaces {
			interfaceMetrics := make(definition.RawMetrics)
			AddUint64RawMetric(interfaceMetrics, "rxBytes", i.RxBytes)
			AddUint64RawMetric(interfaceMetrics, "txBytes", i.TxBytes)
			if i.RxErrors != nil && i.TxErrors != nil {
				interfaceMetrics["errors"] = *i.RxErrors + *i.TxErrors
			}
			interfaces[i.Name] = interfaceMetrics
		}
		r["interfaces"] = interfaces
	}

	rawEntityID := fmt.Sprintf("%s_%s", r["namespace"], r["podName"])

	return r, rawEntityID, nil
}

func fetchContainerStats(c v1.ContainerStats) (definition.RawMetrics, error) {
	r := make(definition.RawMetrics)

	if c.Name == "" {
		return r, fmt.Errorf("empty container identifier, possible data error in %s response", StatsSummaryPath)
	}
	r["containerName"] = c.Name

	if c.CPU != nil {
		AddUint64RawMetric(r, "usageNanoCores", c.CPU.UsageNanoCores)
	}
	if c.Memory != nil {
		AddUint64RawMetric(r, "usageBytes", c.Memory.UsageBytes)
		AddUint64RawMetric(r, "workingSetBytes", c.Memory.WorkingSetBytes)
	}
	if c.Rootfs != nil {
		AddUint64RawMetric(r, "fsAvailableBytes", c.Rootfs.AvailableBytes)
		AddUint64RawMetric(r, "fsCapacityBytes", c.Rootfs.CapacityBytes)
		AddUint64RawMetric(r, "fsUsedBytes", c.Rootfs.UsedBytes)
		AddUint64RawMetric(r, "fsInodesFree", c.Rootfs.InodesFree)
		AddUint64RawMetric(r, "fsInodes", c.Rootfs.Inodes)
		AddUint64RawMetric(r, "fsInodesUsed", c.Rootfs.InodesUsed)
	}

	return r, nil
}

func fetchVolumeStats(v v1.VolumeStats) (definition.RawMetrics, error) {
	r := make(definition.RawMetrics)

	if v.Name == "" {
		return r, fmt.Errorf("empty volume identifier, possible data error in %s response", StatsSummaryPath)
	}
	r["volumeName"] = v.Name
	if v.PVCRef != nil {
		r["pvcName"] = v.PVCRef.Name
		r["pvcNamespace"] = v.PVCRef.Namespace
	}

	AddUint64RawMetric(r, "fsAvailableBytes", v.FsStats.AvailableBytes)
	AddUint64RawMetric(r, "fsCapacityBytes", v.FsStats.CapacityBytes)
	AddUint64RawMetric(r, "fsUsedBytes", v.FsStats.UsedBytes)
	AddUint64RawMetric(r, "fsInodesFree", v.FsStats.InodesFree)
	AddUint64RawMetric(r, "fsInodes", v.FsStats.Inodes)
	AddUint64RawMetric(r, "fsInodesUsed", v.FsStats.InodesUsed)

	return r, nil
}

// GroupStatsSummary groups specific data for pods, containers and node
func GroupStatsSummary(statsSummary *v1.Summary) (definition.RawGroups, []error) {
	if statsSummary == nil {
		return nil, []error{fmt.Errorf("got nil stats summary")}
	}

	var errs []error
	var rawEntityID string
	g := definition.RawGroups{
		"pod":       {},
		"container": {},
		"volume":    {},
		"node":      {},
	}

	rawNodeData, rawEntityID, err := fetchNodeStats(statsSummary.Node)
	if err != nil {
		errs = append(errs, err)
	} else {
		g["node"][rawEntityID] = rawNodeData
	}

	if statsSummary.Pods == nil {
		errs = append(errs, fmt.Errorf("pods data not found, possible data error in %s response", StatsSummaryPath))
		return g, errs
	}

	for _, pod := range statsSummary.Pods {
		rawPodMetrics, rawEntityID, err := fetchPodStats(pod)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		g["pod"][rawEntityID] = rawPodMetrics
		for _, volume := range pod.VolumeStats {
			rawVolumeMetrics, err := fetchVolumeStats(volume)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			rawVolumeMetrics["podName"] = rawPodMetrics["podName"]
			rawVolumeMetrics["namespace"] = rawPodMetrics["namespace"]
			rawEntityID = fmt.Sprintf("%s_%s_%s", rawPodMetrics["namespace"], rawPodMetrics["podName"], rawVolumeMetrics["volumeName"])
			g["volume"][rawEntityID] = rawVolumeMetrics
		}

		for _, container := range pod.Containers {
			rawContainerMetrics, err := fetchContainerStats(container)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			rawContainerMetrics["podName"] = rawPodMetrics["podName"]
			rawContainerMetrics["namespace"] = rawPodMetrics["namespace"]

			rawEntityID = fmt.Sprintf("%s_%s_%s", rawPodMetrics["namespace"], rawPodMetrics["podName"], rawContainerMetrics["containerName"])

			g["container"][rawEntityID] = rawContainerMetrics
		}
	}

	return g, errs
}

// FromRawGroupsEntityIDGenerator generates an entityID from the pod name from kubelet. It's only used for k8s containers.
func FromRawGroupsEntityIDGenerator(key string) definition.EntityIDGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		v, ok := g[groupLabel][rawEntityID][key]
		if !ok {
			return "", fmt.Errorf("%q not found for %q", key, groupLabel)
		}

		val, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("incorrect type of %q for %q", key, groupLabel)
		}
		return val, nil
	}
}

// FromRawEntityIDGroupEntityIDGenerator generates an entityID from the raw entity ID
// which is composed of namespace and pod name. It's used only for k8s pods.
func FromRawEntityIDGroupEntityIDGenerator(key string) definition.EntityIDGeneratorFunc {
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		toRemove, ok := g[groupLabel][rawEntityID][key]
		if !ok {
			return "", fmt.Errorf("%q not found for %q", key, groupLabel)
		}
		v := strings.TrimPrefix(rawEntityID, fmt.Sprintf("%s_", toRemove))

		if v == "" {
			return "", errors.New("generated entity ID is empty")
		}

		return v, nil
	}
}

// FromRawGroupsEntityTypeGenerator generates the entity type using the cluster name and group label.
// If group label is different than "namespace" or "node", then entity type is also composed of namespace.
// If group label is "container" then pod name is also included.
func FromRawGroupsEntityTypeGenerator(groupLabel string, rawEntityID string, groups definition.RawGroups, clusterName string) (string, error) {
	switch groupLabel {
	case "namespace", "node":
		return fmt.Sprintf("k8s:%s:%s", clusterName, groupLabel), nil

	case "container":
		keys, err := getKeys(groupLabel, rawEntityID, groups, "namespace", "podName")
		if err != nil {
			return "", err
		}
		if len(keys) != 2 {
			return "", fmt.Errorf("cannot retrieve values for composing entity type for %q", groupLabel)
		}
		namespace := keys[0]
		podName := keys[1]
		if namespace == "" || podName == "" {
			return "", fmt.Errorf("empty values for generated entity type for %q", groupLabel)
		}
		return fmt.Sprintf("k8s:%s:%s:%s:%s", clusterName, namespace, podName, groupLabel), nil
	default:
		keys, err := getKeys(groupLabel, rawEntityID, groups, "namespace")
		if err != nil {
			return "", err
		}
		if len(keys) == 0 {
			return "", fmt.Errorf("cannot retrieve namespace for composing entity type for %q", groupLabel)
		}
		namespace := keys[0]
		if namespace == "" {
			return "", fmt.Errorf("empty namespace for generated entity type for %q", groupLabel)
		}
		return fmt.Sprintf("k8s:%s:%s:%s", clusterName, namespace, groupLabel), nil
	}
}

func FromLabelGetNamespace(metrics definition.RawMetrics) string {
	if ns, ok := metrics["namespace"].(string); ok {
		return ns
	}
	return ""
}

func getKeys(groupLabel, rawEntityID string, groups definition.RawGroups, keys ...string) ([]string, error) {
	var s []string
	gl, ok := groups[groupLabel]
	if !ok {
		return s, fmt.Errorf("%q not found", groupLabel)
	}
	en, ok := gl[rawEntityID]
	if !ok {
		return s, fmt.Errorf("entity data %q not found for %q", rawEntityID, groupLabel)
	}

	for _, key := range keys {
		v, ok := en[key]
		if !ok {
			return s, fmt.Errorf("%q not found for %q", key, groupLabel)
		}

		val, ok := v.(string)
		if !ok {
			return s, fmt.Errorf("incorrect type of %q for %q", key, groupLabel)
		}

		s = append(s, val)
	}

	return s, nil
}

// AddUint64RawMetric adds a new metric to a RawMetrics if it exists
func AddUint64RawMetric(r definition.RawMetrics, name string, valuePtr *uint64) {
	if valuePtr != nil {
		r[name] = *valuePtr
	}
}
