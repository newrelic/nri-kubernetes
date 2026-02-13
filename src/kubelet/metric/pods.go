package metric

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// KubeletPodsPath is the path where kubelet serves information about pods.
const KubeletPodsPath = "/pods"
const KubeServiceKubeletPodsPath = "/api/v1/pods"
const nodeSelectorQuery = "fieldSelector=spec.nodeName=%s"

// PodsFetcher queries the kubelet and fetches the information of pods
// running on the node. It contains an in-memory cache to store the
// results and avoid querying the kubelet multiple times in the same
// integration execution.
type PodsFetcher struct {
	logger         *log.Logger
	client         client.HTTPGetter
	useKubeService bool
	uri            url.URL
}

// DoPodsFetch used to have a cache that was invalidated each execution of the integration
// TODO: could we move this to informers?
func (podsFetcher *PodsFetcher) DoPodsFetch() (definition.RawGroups, error) {
	podsFetcher.logger.Debugf("Retrieving the list of pods")

	r, err := podsFetcher.Fetch()
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = r.Body.Close()
	}()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error calling kubelet %s path. Status code %d", KubeletPodsPath, r.StatusCode)
	}

	// Cap at 100MB to prevent OOM from a misconfigured or compromised kubelet.
	// Normal /pods responses are typically 1-10MB even on large nodes.
	rawPods, err := io.ReadAll(io.LimitReader(r.Body, 100<<20))
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

	return podsFetcher.fillGaps(raw, pods), nil
}

func (podsFetcher *PodsFetcher) fillGaps(raw definition.RawGroups, pods v1.PodList) definition.RawGroups { //nolint:gocyclo,cyclop
	// If missing, we get the nodeIP from any other container in the node.
	// Due to Kubelet "Wrong Pending status" bug. See https://github.com/kubernetes/kubernetes/pull/57106
	var missingNodeIPContainerIDs []string
	var missingNodeIPPodIDs []string
	var nodeIP string

	for _, p := range pods.Items {
		id := podID(&p)
		raw["pod"][id] = podsFetcher.fetchPodData(&p)

		if _, ok := raw["pod"][id]["nodeIP"]; ok && nodeIP == "" {
			nodeIP = raw["pod"][id]["nodeIP"].(string)
		}

		if nodeIP == "" {
			missingNodeIPPodIDs = append(missingNodeIPPodIDs, id)
		} else {
			raw["pod"][id]["nodeIP"] = nodeIP
		}

		containers := podsFetcher.fetchContainersData(&p)
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

	return raw
}

func (podsFetcher *PodsFetcher) Fetch() (*http.Response, error) {
	if podsFetcher.useKubeService {
		return podsFetcher.client.GetURI(podsFetcher.uri) //nolint:wrapcheck
	}
	return podsFetcher.client.Get(KubeletPodsPath) //nolint:wrapcheck
}

// NewPodsFetcher returns a new PodsFetcher.
func NewBasicPodsFetcher(l *log.Logger, c client.HTTPGetter) *PodsFetcher {
	return &PodsFetcher{
		logger:         l,
		client:         c,
		useKubeService: false,
	}
}

// NewPodsFetcher returns a new PodsFetcher.
func NewPodsFetcher(log *log.Logger, c client.HTTPGetter, config *config.Config) *PodsFetcher {
	if config.FetchPodsFromKubeService {
		log.Info("Using Kubernetes service to fetch pods.")

		uri, _ := url.Parse(getKubeServiceHost())
		uri.Path = path.Join(uri.Path, KubeServiceKubeletPodsPath)
		uri.RawQuery = fmt.Sprintf(nodeSelectorQuery, config.NodeName)

		return &PodsFetcher{
			logger:         log,
			client:         c,
			uri:            *uri,
			useKubeService: true,
		}
	}

	return &PodsFetcher{
		logger:         log,
		client:         c,
		useKubeService: false,
	}
}

func getKubeServiceHost() string {
	inClusterConfig, err := rest.InClusterConfig()

	if err == nil {
		return inClusterConfig.Host
	}

	return fmt.Sprintf("https://%s:%s", os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")) //nolint: nosprintfhostport
}

func (podsFetcher *PodsFetcher) fetchContainersData(pod *v1.Pod) map[string]definition.RawMetrics {
	statuses := make(map[string]definition.RawMetrics)
	fillContainerStatuses(pod, statuses)

	metrics := make(map[string]definition.RawMetrics)
	containers := pod.Spec.Containers

	// Add sidecar containers
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.RestartPolicy != nil && *initContainer.RestartPolicy == v1.ContainerRestartPolicyAlways {
			containers = append(containers, initContainer)
		}
	}

	for _, c := range containers {
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
			creatorKind := ref[0].Kind
			creatorName := ref[0].Name
			addWorkloadNameBasedOnCreator(creatorKind, creatorName, metrics[id])
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
	containerStatuses := pod.Status.ContainerStatuses

	// Add sidecar containers
	for idx, initContainer := range pod.Spec.InitContainers {
		if initContainer.RestartPolicy != nil && *initContainer.RestartPolicy == v1.ContainerRestartPolicyAlways {
			containerStatuses = append(containerStatuses, pod.Status.InitContainerStatuses[idx])
		}
	}

	for _, c := range containerStatuses {
		name := c.Name
		id := containerID(pod, name)

		// Set the ExitCode. Zero if no terminated Exit Code.
		var lastTerminatedExitCode int32
		lastTerminatedExitReason := "None"
		var lastTerminatedFinishedAt time.Time
		if c.LastTerminationState.Terminated != nil {
			lastTerminatedExitCode = c.LastTerminationState.Terminated.ExitCode
			lastTerminatedExitReason = c.LastTerminationState.Terminated.Reason
			lastTerminatedFinishedAt = c.LastTerminationState.Terminated.FinishedAt.Time.In(time.UTC)
		}

		dest[id] = make(definition.RawMetrics)

		switch {
		case c.State.Running != nil:
			dest[id]["status"] = "Running"
			dest[id]["startedAt"] = c.State.Running.StartedAt.Time.In(time.UTC) // TODO WE DO NOT REPORT THAT METRIC
			dest[id]["restartCount"] = c.RestartCount
			dest[id]["isReady"] = c.Ready
			dest[id]["lastTerminatedExitCode"] = lastTerminatedExitCode
			dest[id]["lastTerminatedExitReason"] = lastTerminatedExitReason
			dest[id]["lastTerminatedTimestamp"] = lastTerminatedFinishedAt
		case c.State.Waiting != nil:
			dest[id]["status"] = "Waiting"
			dest[id]["reason"] = c.State.Waiting.Reason
			dest[id]["restartCount"] = c.RestartCount
			dest[id]["lastTerminatedExitCode"] = lastTerminatedExitCode
			dest[id]["lastTerminatedExitReason"] = lastTerminatedExitReason
			dest[id]["lastTerminatedTimestamp"] = lastTerminatedFinishedAt
		case c.State.Terminated != nil:
			dest[id]["status"] = "Terminated"
			dest[id]["reason"] = c.State.Terminated.Reason
			dest[id]["restartCount"] = c.RestartCount
			dest[id]["lastTerminatedExitCode"] = lastTerminatedExitCode
			dest[id]["lastTerminatedExitReason"] = lastTerminatedExitReason
			dest[id]["lastTerminatedTimestamp"] = lastTerminatedFinishedAt
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
func (podsFetcher *PodsFetcher) fetchPodData(pod *v1.Pod) definition.RawMetrics {
	metrics := definition.RawMetrics{
		"namespace": pod.GetObjectMeta().GetNamespace(),
		"podName":   pod.GetObjectMeta().GetName(),
		"nodeName":  pod.Spec.NodeName,
	}

	podsFetcher.fillPodStatus(metrics, pod)

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
		creatorKind := ref[0].Kind
		creatorName := ref[0].Name
		metrics["createdKind"] = creatorKind
		metrics["createdBy"] = creatorName
		addWorkloadNameBasedOnCreator(creatorKind, creatorName, metrics)
	}

	if pod.Status.Reason != "" {
		metrics["reason"] = pod.Status.Reason
	}

	if pod.Status.Message != "" {
		metrics["message"] = pod.Status.Message
	}

	// Priority for eviction analysis
	if pod.Spec.Priority != nil {
		metrics["priority"] = *pod.Spec.Priority
	}

	if pod.Spec.PriorityClassName != "" {
		metrics["priorityClassName"] = pod.Spec.PriorityClassName
	}

	labels := podLabels(pod)
	if len(labels) > 0 {
		metrics["labels"] = labels
	}

	return metrics
}

func (podsFetcher *PodsFetcher) fillPodStatus(r definition.RawMetrics, pod *v1.Pod) { //nolint:gocognit,gocyclo,cyclop
	// TODO Review if those Fake Pending Pods are still an issue
	if isFakePendingPod(pod.Status) {
		r["status"] = "Running"
		r["isReady"] = "True"
		r["isScheduled"] = "True"

		podsFetcher.logger.Debugf("Fake Pending Pod marked as Running")

		return
	}

	for _, c := range pod.Status.Conditions {
		switch c.Type {
		case "Initialized":
			if c.Status == "True" {
				if !c.LastTransitionTime.IsZero() {
					r["initializedAt"] = c.LastTransitionTime.In(time.UTC)
				}
			}
		case "Ready":
			r["isReady"] = string(c.Status)
			if c.Status == "True" {
				if !c.LastTransitionTime.IsZero() {
					r["readyAt"] = c.LastTransitionTime.In(time.UTC)
				}
			}
		case "ContainersReady":
			if c.Status == "True" {
				if !c.LastTransitionTime.IsZero() {
					r["containersReadyAt"] = c.LastTransitionTime.In(time.UTC)
				}
			}
		case "PodScheduled":
			r["isScheduled"] = string(c.Status)
			if c.Status == "True" {
				if !c.LastTransitionTime.IsZero() {
					r["scheduledAt"] = c.LastTransitionTime.In(time.UTC)
				}
			}
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

func addWorkloadNameBasedOnCreator(creatorKind string, creatorName string, metrics definition.RawMetrics) {
	switch creatorKind {
	case "DaemonSet":
		metrics["daemonsetName"] = creatorName
	case "Deployment":
		metrics["deploymentName"] = creatorName
	case "Job":
		metrics["jobName"] = creatorName
	case "ReplicaSet":
		metrics["replicasetName"] = creatorName
		if d := deploymentNameBasedOnCreator(creatorKind, creatorName); d != "" {
			metrics["deploymentName"] = d
		}
	case "StatefulSet":
		metrics["statefulsetName"] = creatorName
	}
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
