package testdata

import (
	"time"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// ExpectedRawData is the expectation for main fetch_test tests.
var ExpectedRawData = definition.RawGroups{
	"pod": {
		"kube-system_kube-controller-manager-minikube": {
			"nodeName":          "minikube",
			"isReady":           "True",
			"isScheduled":       "True",
			"nodeIP":            "192.168.99.100",
			"podIP":             "10.0.2.15",
			"labels":            map[string]string{"k8s-app": "kube-controller-manager", "component": "kube-controller-manager", "tier": "control-plane"},
			"namespace":         "kube-system",
			"podName":           "kube-controller-manager-minikube",
			"priority":          int32(2000000000),
			"priorityClassName": "system-cluster-critical",
			"status":            "Running",
			"startTime":         parseTime("2019-10-23T17:10:48Z"),
			"containersReadyAt": parseTime("2019-10-23T17:10:49Z"),
			"initializedAt":     parseTime("2019-10-23T17:10:48Z"),
			"readyAt":           parseTime("2019-10-23T17:10:49Z"),
			"scheduledAt":       parseTime("2019-10-23T17:10:48Z"),
		},
		"kube-system_newrelic-infra-rz225": {
			"createdKind":   "DaemonSet",
			"createdBy":     "newrelic-infra",
			"nodeIP":        "192.168.99.100",
			"podIP":         "172.17.0.3",
			"namespace":     "kube-system",
			"daemonsetName": "newrelic-infra",
			"podName":       "newrelic-infra-rz225",
			"nodeName":      "minikube",
			"startTime":     parseTime("2018-02-14T16:26:33Z"),
			"status":        "Running",
			"isReady":       "True",
			"isScheduled":   "True",
			"createdAt":     parseTime("2018-02-14T16:26:33Z"),
			"initializedAt": parseTime("2018-02-14T16:26:33Z"),
			"readyAt":       parseTime("2018-02-27T15:21:18Z"),
			"scheduledAt":   parseTime("2018-02-14T16:27:00Z"),
			"labels": map[string]string{
				"controller-revision-hash": "3887482659",
				"name":                     "newrelic-infra",
				"pod-template-generation":  "1",
			},
		},
		"kube-system_kube-state-metrics-57f4659995-6n2qq": {
			"createdKind":    "ReplicaSet",
			"createdBy":      "kube-state-metrics-57f4659995",
			"nodeIP":         "192.168.99.100",
			"namespace":      "kube-system",
			"podName":        "kube-state-metrics-57f4659995-6n2qq",
			"nodeName":       "minikube",
			"status":         "Running", // Running because is fake pending pod.
			"isReady":        "True",
			"isScheduled":    "True",
			"createdAt":      parseTime("2018-02-14T16:27:38Z"),
			"deploymentName": "kube-state-metrics",
			"replicasetName": "kube-state-metrics-57f4659995",
			"labels": map[string]string{
				"k8s-app":           "kube-state-metrics",
				"pod-template-hash": "1390215551",
			},
		},
		"default_sh-7c95664875-4btqh": {
			"createdKind":    "ReplicaSet",
			"createdBy":      "sh-7c95664875",
			"nodeIP":         "192.168.99.100",
			"namespace":      "default",
			"podName":        "sh-7c95664875-4btqh",
			"nodeName":       "minikube",
			"status":         "Failed",
			"reason":         "Evicted",
			"message":        "The node was low on resource: memory.",
			"createdAt":      parseTime("2019-03-13T07:59:00Z"),
			"startTime":      parseTime("2019-03-13T07:59:00Z"),
			"deploymentName": "sh",
			"replicasetName": "sh-7c95664875",
			"labels": map[string]string{
				"pod-template-hash": "3751220431",
				"run":               "sh",
			},
		},
	},
	"container": {
		"kube-system_newrelic-infra-rz225_newrelic-infra": {
			"containerName":  "newrelic-infra",
			"containerImage": "newrelic/ohaik:1.0.0-beta3",
			"namespace":      "kube-system",
			"podName":        "newrelic-infra-rz225",
			"daemonsetName":  "newrelic-infra",
			"nodeName":       "minikube",
			"nodeIP":         "192.168.99.100",
			"restartCount":   int32(6),
			"isReady":        true,
			"status":         "Running",
			// "reason":               "", // TODO
			"startedAt":                parseTime("2018-02-27T15:21:16Z"),
			"lastTerminatedExitCode":   int32(0),
			"lastTerminatedExitReason": "Completed",
			"lastTerminatedTimestamp":  parseTime("2018-02-27T15:21:10Z"),
			"cpuRequestedCores":        int64(100),
			"memoryRequestedBytes":     int64(104857600),
			"memoryLimitBytes":         int64(104857600),
			"labels": map[string]string{
				"controller-revision-hash": "3887482659",
				"name":                     "newrelic-infra",
				"pod-template-generation":  "1",
			},
		},
		"kube-system_kube-state-metrics-57f4659995-6n2qq_kube-state-metrics": {
			"containerName":  "kube-state-metrics",
			"containerImage": "quay.io/coreos/kube-state-metrics:v1.1.0",
			"namespace":      "kube-system",
			"podName":        "kube-state-metrics-57f4659995-6n2qq",
			"replicasetName": "kube-state-metrics-57f4659995",
			"nodeName":       "minikube",
			"nodeIP":         "192.168.99.100",
			// "restartCount":   int32(7),  // No restartCount since there is no restartCount in status field in the pod.
			// "isReady":        false,     // No isReady since there is no isReady in status field in the pod.
			// "status":         "Running", // No Status since there is no ContainerStatuses field in the pod.
			"deploymentName": "kube-state-metrics",
			// "startedAt":            parseTime("2018-02-27T15:21:37Z"), // No startedAt since there is no startedAt in status field in the pod.
			"cpuRequestedCores":    int64(101),
			"cpuLimitCores":        int64(101),
			"memoryRequestedBytes": int64(106954752),
			"memoryLimitBytes":     int64(106954752),
			"labels": map[string]string{
				"k8s-app":           "kube-state-metrics",
				"pod-template-hash": "1390215551",
			},
		},
		"kube-system_kube-state-metrics-57f4659995-6n2qq_addon-resizer": {
			"containerName":  "addon-resizer",
			"containerImage": "gcr.io/google_containers/addon-resizer:1.0",
			"namespace":      "kube-system",
			"podName":        "kube-state-metrics-57f4659995-6n2qq",
			"replicasetName": "kube-state-metrics-57f4659995",
			"nodeName":       "minikube",
			"nodeIP":         "192.168.99.100",
			// "restartCount":   int32(7),  // No restartCount since there is no restartCount in status field in the pod.
			// "isReady":        false,     // No isReady since there is no isReady in status field in the pod.
			// "status":         "Running", // No Status since there is no ContainerStatuses field in the pod.
			"deploymentName": "kube-state-metrics",
			// "reason":               "",                                // TODO
			// "startedAt":            parseTime("2018-02-27T15:21:38Z"), // No startedAt since there is no startedAt in status field in the pod.
			"cpuRequestedCores":    int64(100),
			"cpuLimitCores":        int64(100),
			"memoryRequestedBytes": int64(31457280),
			"memoryLimitBytes":     int64(31457280),
			"labels": map[string]string{
				"k8s-app":           "kube-state-metrics",
				"pod-template-hash": "1390215551",
			},
		},
		"default_sh-7c95664875-4btqh_sh": {
			"containerName":  "sh",
			"containerImage": "python",
			"namespace":      "default",
			"podName":        "sh-7c95664875-4btqh",
			"replicasetName": "sh-7c95664875",
			"nodeName":       "minikube",
			"nodeIP":         "192.168.99.100",
			"deploymentName": "sh",
			"labels": map[string]string{
				"pod-template-hash": "3751220431",
				"run":               "sh",
			},
		},

		"kube-system_kube-controller-manager-minikube_kube-controller-manager": {
			"nodeName": "minikube",
			"isReady":  bool(true),
			"labels": map[string]string{
				"tier":      "control-plane",
				"k8s-app":   "kube-controller-manager",
				"component": "kube-controller-manager",
			},
			"podName":                  "kube-controller-manager-minikube",
			"containerImage":           "k8s.gcr.io/kube-controller-manager:v1.16.0",
			"namespace":                "kube-system",
			"nodeIP":                   "192.168.99.100",
			"cpuRequestedCores":        int64(200),
			"status":                   "Running",
			"lastTerminatedExitCode":   int32(255),
			"lastTerminatedExitReason": "Error",
			"lastTerminatedTimestamp":  parseTime("2019-10-23T17:10:25Z"),
			"startedAt":                parseTime("2019-10-23T17:10:49Z"),
			"restartCount":             int32(1),
			"containerName":            "kube-controller-manager",
		},
	},
}

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339, raw)

	return t
}
