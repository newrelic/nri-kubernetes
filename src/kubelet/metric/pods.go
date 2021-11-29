package metric

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	v1 "k8s.io/api/core/v1"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/definition"
)

// KubeletPodsPath is the path where kubelet serves information about pods.
const KubeletPodsPath = "/pods"

// PodsFetcher queries the kubelet and fetches the information of pods
// running on the node. It contains an in-memory cache to store the
// results and avoid querying the kubelet multiple times in the same
// integration execution.
type PodsFetcher struct {
	logger log.Logger
	client client.HTTPGetter
}

// DoPodsFetch used to have a cache that was invalidated each execution of the integration
// TODO: could we move this to informers?
func (f *PodsFetcher) DoPodsFetch() (definition.RawGroups, error) {
	f.logger.Debugf("Retrieving the list of pods")

	r, err := f.client.Get(KubeletPodsPath)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = r.Body.Close()
	}()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error calling kubelet %s path. Status code %d", KubeletPodsPath, r.StatusCode)
	}

	rawPods, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from kubelet %s path. %s", KubeletPodsPath, err)
	}

	if len(rawPods) == 0 {
		return nil, fmt.Errorf("error reading response from kubelet %s path. Response is empty", KubeletPodsPath)
	}

	// v1.PodList comes from k8s api core library.
	var pods v1.PodList
	err = json.Unmarshal(rawPods, &pods)
	if err != nil {
		return nil, fmt.Errorf("error decoding response from kubelet %s path. %s", KubeletPodsPath, err)
	}

	raw := definition.RawGroups{
		"pod":       make(map[string]definition.RawMetrics),
		"container": make(map[string]definition.RawMetrics),
	}

	// If missing, we get the nodeIP from any other container in the node.
	// Due to Kubelet "Wrong Pending status" bug. See https://github.com/kubernetes/kubernetes/pull/57106
	var missingNodeIPContainerIDs []string
	var missingNodeIPPodIDs []string
	var nodeIP string

	for _, p := range pods.Items {
		id := podID(&p)
		raw["pod"][id] = f.fetchPodData(&p)

		if _, ok := raw["pod"][id]["nodeIP"]; ok && nodeIP == "" {
			nodeIP = raw["pod"][id]["nodeIP"].(string)
		}

		if nodeIP == "" {
			missingNodeIPPodIDs = append(missingNodeIPPodIDs, id)
		} else {
			raw["pod"][id]["nodeIP"] = nodeIP
		}

		containers := f.fetchContainersData(&p)
		for id, c := range containers {
			raw["container"][id] = c

			if _, ok := c["nodeIP"]; ok && nodeIP == "" {
				nodeIP = c["nodeIP"].(string)
			}

			if nodeIP == "" {
				missingNodeIPContainerIDs = append(missingNodeIPContainerIDs, id)
			} else {
				raw["container"][id]["nodeIP"] = nodeIP
			}
		}
	}

	for _, id := range missingNodeIPPodIDs {
		raw["pod"][id]["nodeIP"] = nodeIP
	}

	for _, id := range missingNodeIPContainerIDs {
		raw["container"][id]["nodeIP"] = nodeIP
	}

	return raw, nil
}

// NewPodsFetcher returns a new PodsFetcher.
func NewPodsFetcher(l log.Logger, c client.HTTPGetter) *PodsFetcher {
	return &PodsFetcher{
		logger: l,
		client: c,
	}
}

func (f *PodsFetcher) fetchContainersData(pod *v1.Pod) map[string]definition.RawMetrics {
	statuses := make(map[string]definition.RawMetrics)
	fillContainerStatuses(pod, statuses)

	metrics := make(map[string]definition.RawMetrics)

	for _, c := range pod.Spec.Containers {
		id := containerID(pod, c.Name)
		metrics[id] = definition.RawMetrics{
			"containerName":  c.Name,
			"containerImage": c.Image,
			"namespace":      pod.GetObjectMeta().GetNamespace(),
			"podName":        pod.GetObjectMeta().GetName(),
			"nodeName":       pod.Spec.NodeName,
		}

		if v := pod.Status.HostIP; v != "" {
			metrics[id]["nodeIP"] = v
		}

		if v, ok := c.Resources.Requests[v1.ResourceCPU]; ok {
			metrics[id]["cpuRequestedCores"] = v.MilliValue()
		}

		if v, ok := c.Resources.Limits[v1.ResourceCPU]; ok {
			metrics[id]["cpuLimitCores"] = v.MilliValue()
		}

		if v, ok := c.Resources.Requests[v1.ResourceMemory]; ok {
			metrics[id]["memoryRequestedBytes"] = v.Value()
		}

		if v, ok := c.Resources.Limits[v1.ResourceMemory]; ok {
			metrics[id]["memoryLimitBytes"] = v.Value()
		}

		if ref := pod.GetOwnerReferences(); len(ref) > 0 {
			if d := deploymentNameBasedOnCreator(ref[0].Kind, ref[0].Name); d != "" {
				metrics[id]["deploymentName"] = d
			}
		}

		// merging status data
		for k, v := range statuses[id] {
			metrics[id][k] = v
		}

		labels := podLabels(pod)
		if len(labels) > 0 {
			metrics[id]["labels"] = labels
		}
	}

	return metrics
}

func fillContainerStatuses(pod *v1.Pod, dest map[string]definition.RawMetrics) {
	for _, c := range pod.Status.ContainerStatuses {
		name := c.Name
		id := containerID(pod, name)

		dest[id] = make(definition.RawMetrics)

		switch {
		case c.State.Running != nil:
			dest[id]["status"] = "Running"
			dest[id]["startedAt"] = c.State.Running.StartedAt.Time.In(time.UTC) // TODO WE DO NOT REPORT THAT METRIC
			dest[id]["restartCount"] = c.RestartCount
			dest[id]["isReady"] = c.Ready
		case c.State.Waiting != nil:
			dest[id]["status"] = "Waiting"
			dest[id]["reason"] = c.State.Waiting.Reason
			dest[id]["restartCount"] = c.RestartCount
		case c.State.Terminated != nil:
			dest[id]["status"] = "Terminated"
			dest[id]["reason"] = c.State.Terminated.Reason
			dest[id]["restartCount"] = c.RestartCount
			dest[id]["startedAt"] = c.State.Terminated.StartedAt.Time.In(time.UTC) // TODO WE DO NOT REPORT THAT METRIC
		default:
			dest[id]["status"] = "Unknown"
		}
	}
}

// isFakePendingPods returns true if a pod is a fake pending pod.
// Pods that are created before having API server up are reported as Pending
// in Kubelet /pods endpoint where in fact they are correctly running. This is a bug in Kubelet.
// Those pods are called fake pending pods.
func isFakePendingPod(s v1.PodStatus) bool {
	return s.Phase == "Pending" &&
		len(s.Conditions) == 1 &&
		s.Conditions[0].Type == "PodScheduled" &&
		s.Conditions[0].Status == "True"
}

// TODO handle errors and missing data
func (f *PodsFetcher) fetchPodData(pod *v1.Pod) definition.RawMetrics {
	metrics := definition.RawMetrics{
		"namespace": pod.GetObjectMeta().GetNamespace(),
		"podName":   pod.GetObjectMeta().GetName(),
		"nodeName":  pod.Spec.NodeName,
	}

	f.fillPodStatus(metrics, pod)

	if v := pod.Status.HostIP; v != "" {
		metrics["nodeIP"] = v
	}

	// IP address allocated to the pod. Routable at least within the cluster. Empty if not yet allocated.
	if podIP := pod.Status.PodIP; podIP != "" {
		metrics["podIP"] = podIP
	}

	if pod.Status.StartTime != nil {
		metrics["startTime"] = pod.Status.StartTime.Time.In(time.UTC)
	}

	if t := pod.GetObjectMeta().GetCreationTimestamp(); !t.IsZero() {
		metrics["createdAt"] = t.In(time.UTC)
	}

	if ref := pod.GetOwnerReferences(); len(ref) > 0 {
		metrics["createdKind"] = ref[0].Kind
		metrics["createdBy"] = ref[0].Name
		if d := deploymentNameBasedOnCreator(ref[0].Kind, ref[0].Name); d != "" {
			metrics["deploymentName"] = d
		}
	}

	if pod.Status.Reason != "" {
		metrics["reason"] = pod.Status.Reason
	}

	if pod.Status.Message != "" {
		metrics["message"] = pod.Status.Message
	}

	labels := podLabels(pod)
	if len(labels) > 0 {
		metrics["labels"] = labels
	}

	return metrics
}

func (f *PodsFetcher) fillPodStatus(r definition.RawMetrics, pod *v1.Pod) {
	// TODO Review if those Fake Pending Pods are still an issue
	if isFakePendingPod(pod.Status) {
		r["status"] = "Running"
		r["isReady"] = "True"
		r["isScheduled"] = "True"

		f.logger.Debugf("Fake Pending Pod marked as Running")

		return
	}

	for _, c := range pod.Status.Conditions {
		switch c.Type {
		case "Ready":
			r["isReady"] = string(c.Status)
		case "PodScheduled":
			r["isScheduled"] = string(c.Status)
		}
	}

	r["status"] = string(pod.Status.Phase)
}

func podLabels(p *v1.Pod) map[string]string {
	labels := make(map[string]string, len(p.GetObjectMeta().GetLabels()))
	for k, v := range p.GetObjectMeta().GetLabels() {
		labels[k] = v
	}

	return labels
}

func deploymentNameBasedOnCreator(creatorKind, creatorName string) string {
	var deploymentName string
	if creatorKind == "ReplicaSet" {
		deploymentName = replicasetNameToDeploymentName(creatorName)
	}
	return deploymentName
}

func replicasetNameToDeploymentName(rsName string) string {
	s := strings.Split(rsName, "-")
	return strings.Join(s[:len(s)-1], "-")
}

func podID(pod *v1.Pod) string {
	return fmt.Sprintf("%v_%v", pod.GetObjectMeta().GetNamespace(), pod.GetObjectMeta().GetName())
}

func containerID(pod *v1.Pod, containerName string) string {
	return fmt.Sprintf("%v_%v", podID(pod), containerName)
}
