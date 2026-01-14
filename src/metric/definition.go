package metric

import (
	"errors"
	"fmt"
	"time"

	sdkMetric "github.com/newrelic/infra-integrations-sdk/data/metric"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	ksmMetric "github.com/newrelic/nri-kubernetes/v3/src/ksm/metric"
	kubeletMetric "github.com/newrelic/nri-kubernetes/v3/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

// Fetch Functions for computed metrics
var (
	workingSetBytes   = definition.FromRaw("workingSetBytes")
	_cpuUsedCores     = definition.TransformAndFilter(definition.FromRaw("usageNanoCores"), fromNano, filterCPUUsedCores) //nolint: gochecknoglobals // significant refactoring
	cpuLimitCores     = definition.Transform(definition.FromRaw("cpuLimitCores"), toCores)
	cpuRequestedCores = definition.Transform(definition.FromRaw("cpuRequestedCores"), toCores)
	processOpenFds    = prometheus.FromValueWithOverriddenName("process_open_fds", "processOpenFds")
	processMaxFds     = prometheus.FromValueWithOverriddenName("process_max_fds", "processMaxFds")
)

// APIServerSpecs are the metric specifications we want to collect
// from the control plane API server.
var APIServerSpecs = definition.SpecGroups{
	"api-server": {
		IDGenerator:   prometheus.FromRawEntityIDGenerator,
		TypeGenerator: prometheus.ControlPlaneComponentTypeGenerator,
		Specs: []definition.Spec{
			{
				Name: "apiserverRequestsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"apiserver_request_total",
					"apiserverRequestsDelta",
					prometheus.IncludeOnlyLabelsFilter("verb", "code"),
				),
				Type: sdkMetric.DELTA,
			},
			{
				Name: "apiserverRequestsRate",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"apiserver_request_total",
					"apiserverRequestsRate",
					prometheus.IncludeOnlyLabelsFilter("verb", "code"),
				),
				Type: sdkMetric.RATE,
			},
			{
				Name: "apiserverCurrentInflightRequestsMutating",
				ValueFunc: prometheus.FromValueWithLabelsFilter(
					"apiserver_current_inflight_requests",
					"apiserverCurrentInflightRequestsMutating",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"request_kind": "mutating",
					}),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name: "apiserverCurrentInflightRequestsReadOnly",
				ValueFunc: prometheus.FromValueWithLabelsFilter(
					"apiserver_current_inflight_requests",
					"apiserverCurrentInflightRequestsReadOnly",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"request_kind": "readOnly",
					}),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name: "restClientRequestsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"rest_client_requests_total",
					"restClientRequestsDelta",
					prometheus.IncludeOnlyLabelsFilter("method", "code"),
				),
				Type: sdkMetric.DELTA,
			},
			{
				Name: "restClientRequestsRate",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"rest_client_requests_total",
					"restClientRequestsRate",
					prometheus.IncludeOnlyLabelsFilter("method", "code"),
				),
				Type: sdkMetric.RATE,
			},
			// etcd_object_counts was deprecated in k8s 1.22 and removed in 1.23 (it is replaced by apiserver_storage_objects)
			{
				Name:      "etcdObjectCounts",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_object_counts", "etcdObjectCounts"),
				Type:      sdkMetric.GAUGE,
				Optional:  true,
			},
			// apiserver_storage_objects was introduced in k8s 1.21 and replaces etcd_object_counts in 1.23
			{
				Name: "apiserverStorageObjects",
				ValueFunc: fetchIfMissing(
					prometheus.FromValueWithOverriddenName("apiserver_storage_objects", "apiserverStorageObjects"),
					prometheus.FromValueWithOverriddenName("etcd_object_counts", "etcdObjectCounts"),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name:      "processResidentMemoryBytes",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_resident_memory_bytes", "processResidentMemoryBytes"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processCpuSecondsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_cpu_seconds_total", "processCpuSecondsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "goThreads",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_threads", "goThreads"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "goGoroutines",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_goroutines", "goGoroutines"),
				Type:      sdkMetric.GAUGE,
			},
		},
	},
}

// APIServerQueries are the queries we will do to the control plane
// API Server in order to fetch all the raw metrics.
var APIServerQueries = []prometheus.Query{
	{
		MetricName: "apiserver_request_total",
	},
	{
		MetricName: "rest_client_requests_total",
	},
	{
		MetricName: "etcd_object_counts",
	},
	{
		MetricName: "apiserver_storage_objects",
	},
	{
		MetricName: "apiserver_current_inflight_requests",
	},
	{
		MetricName: "process_resident_memory_bytes",
	},
	{
		MetricName: "process_cpu_seconds_total",
	},
	{
		MetricName: "go_threads",
	},
	{
		MetricName: "go_goroutines",
	},
}

// ControllerManagerSpecs are the metric specifications we want to collect
// from the control plane controller manager.
var ControllerManagerSpecs = definition.SpecGroups{
	"controller-manager": {
		IDGenerator:   prometheus.FromRawEntityIDGenerator,
		TypeGenerator: prometheus.ControlPlaneComponentTypeGenerator,
		Specs: []definition.Spec{
			{
				Name:      "workqueueAddsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("workqueue_adds_total", "workqueueAddsDelta"),
				Type:      sdkMetric.DELTA,
				Optional:  true,
			},
			{
				Name:      "workqueueDepth",
				ValueFunc: prometheus.FromValueWithOverriddenName("workqueue_depth", "workqueueDepth"),
				Type:      sdkMetric.GAUGE,
				Optional:  true,
			},
			{
				Name:      "workqueueRetriesDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("workqueue_retries_total", "workqueueRetriesDelta"),
				Type:      sdkMetric.DELTA,
				Optional:  true,
			},
			{
				Name: "leaderElectionMasterStatus",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"leader_election_master_status",
					"leaderElectionMasterStatus",
					prometheus.IgnoreLabelsFilter("name"),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name:      "processResidentMemoryBytes",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_resident_memory_bytes", "processResidentMemoryBytes"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processCpuSecondsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_cpu_seconds_total", "processCpuSecondsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "goThreads",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_threads", "goThreads"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "goGoroutines",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_goroutines", "goGoroutines"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name: "nodeCollectorEvictionsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"node_collector_evictions_total",
					"nodeCollectorEvictionsDelta",
					prometheus.IgnoreLabelsFilter("zone"),
				),
				Type: sdkMetric.PDELTA,
			},
		},
	},
}

// ControllerManagerQueries are the queries we will do to the control plane
// controller manager in order to fetch all the raw metrics.
var ControllerManagerQueries = []prometheus.Query{
	{
		MetricName: "workqueue_adds_total",
	},
	{
		MetricName: "workqueue_depth",
	},
	{
		MetricName: "workqueue_retries_total",
	},
	{
		MetricName: "leader_election_master_status",
	},
	{
		MetricName: "process_resident_memory_bytes",
	},
	{
		MetricName: "process_cpu_seconds_total",
	},
	{
		MetricName: "go_threads",
	},
	{
		MetricName: "go_goroutines",
	},
	{
		MetricName: "node_collector_evictions_total",
	},
}

// SchedulerSpecs are the metric specifications we want to collect
// from the control plane scheduler.
var SchedulerSpecs = definition.SpecGroups{
	"scheduler": {
		IDGenerator:   prometheus.FromRawEntityIDGenerator,
		TypeGenerator: prometheus.ControlPlaneComponentTypeGenerator,
		Specs: []definition.Spec{
			{
				Name: "leaderElectionMasterStatus",
				ValueFunc: prometheus.FromValueWithOverriddenName(
					"leader_election_master_status",
					"leaderElectionMasterStatus",
					prometheus.IgnoreLabelsFilter("name"),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name:      "restClientRequestsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("rest_client_requests_total", "restClientRequestsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "restClientRequestsRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("rest_client_requests_total", "restClientRequestsRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "schedulerScheduleAttemptsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("scheduler_schedule_attempts_total", "schedulerScheduleAttemptsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "schedulerScheduleAttemptsRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("scheduler_schedule_attempts_total", "schedulerScheduleAttemptsRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "schedulerSchedulingDurationSeconds",
				ValueFunc: prometheus.FromSummary("scheduler_scheduling_duration_seconds"),
				Type:      sdkMetric.GAUGE,
				Optional:  true,
			},
			{
				Name:      "schedulerPreemptionAttemptsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("scheduler_total_preemption_attempts", "schedulerPreemptionAttemptsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name: "schedulerPendingPodsActive",
				ValueFunc: prometheus.FromValueWithLabelsFilter(
					"scheduler_pending_pods",
					"schedulerPendingPodsActive",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"queue": "active",
					}),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name: "schedulerPendingPodsBackoff",
				ValueFunc: prometheus.FromValueWithLabelsFilter(
					"scheduler_pending_pods",
					"schedulerPendingPodsBackoff",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"queue": "backoff",
					}),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name: "schedulerPendingPodsUnschedulable",
				ValueFunc: prometheus.FromValueWithLabelsFilter(
					"scheduler_pending_pods",
					"schedulerPendingPodsUnschedulable",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"queue": "unschedulable",
					}),
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name:      "schedulerPodPreemptionVictims",
				ValueFunc: prometheus.FromValueWithOverriddenName("scheduler_pod_preemption_victims", "schedulerPodPreemptionVictims"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processResidentMemoryBytes",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_resident_memory_bytes", "processResidentMemoryBytes"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processCpuSecondsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_cpu_seconds_total", "processCpuSecondsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "goThreads",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_threads", "goThreads"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "goGoroutines",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_goroutines", "goGoroutines"),
				Type:      sdkMetric.GAUGE,
			},
		},
	},
}

// SchedulerQueries are the queries we will do to the control plane
// scheduler in order to fetch all the raw metrics.
var SchedulerQueries = []prometheus.Query{
	{
		MetricName: "leader_election_master_status",
	},
	{
		MetricName: "rest_client_requests_total",
	},
	{
		MetricName: "scheduler_schedule_attempts_total",
	},
	{
		MetricName: "scheduler_scheduling_duration_seconds",
	},
	{
		MetricName: "scheduler_total_preemption_attempts",
	},
	{
		MetricName: "scheduler_pending_pods",
	},
	{
		MetricName: "scheduler_pod_preemption_victims",
	},
	{
		MetricName: "process_resident_memory_bytes",
	},
	{
		MetricName: "process_cpu_seconds_total",
	},
	{
		MetricName: "go_threads",
	},
	{
		MetricName: "go_goroutines",
	},
}

// EtcdSpecs are the metric specifications we want to collect
// from ETCD.
var EtcdSpecs = definition.SpecGroups{
	"etcd": {
		IDGenerator:   prometheus.FromRawEntityIDGenerator,
		TypeGenerator: prometheus.ControlPlaneComponentTypeGenerator,
		Specs: []definition.Spec{
			{
				Name:      "etcdServerHasLeader",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_has_leader", "etcdServerHasLeader"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "etcdServerLeaderChangesSeenDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_leader_changes_seen_total", "etcdServerLeaderChangesSeenDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "etcdMvccDbTotalSizeInBytes",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_mvcc_db_total_size_in_bytes", "etcdMvccDbTotalSizeInBytes"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "etcdServerProposalsCommittedRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_committed_total", "etcdServerProposalsCommittedRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "etcdServerProposalsCommittedDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_committed_total", "etcdServerProposalsCommittedDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "etcdServerProposalsAppliedRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_applied_total", "etcdServerProposalsAppliedRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "etcdServerProposalsAppliedDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_applied_total", "etcdServerProposalsAppliedDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "etcdServerProposalsPending",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_pending", "etcdServerProposalsPending"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "etcdServerProposalsFailedRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_failed_total", "etcdServerProposalsFailedRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "etcdServerProposalsFailedDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_server_proposals_failed_total", "etcdServerProposalsFailedDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "processOpenFds",
				ValueFunc: processOpenFds,
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processMaxFds",
				ValueFunc: processMaxFds,
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "etcdNetworkClientGrpcReceivedBytesRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_network_client_grpc_received_bytes_total", "etcdNetworkClientGrpcReceivedBytesRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "etcdNetworkClientGrpcSentBytesRate",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_network_client_grpc_sent_bytes_total", "etcdNetworkClientGrpcSentBytesRate"),
				Type:      sdkMetric.RATE,
			},
			{
				Name:      "processResidentMemoryBytes",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_resident_memory_bytes", "processResidentMemoryBytes"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processCpuSecondsDelta",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_cpu_seconds_total", "processCpuSecondsDelta"),
				Type:      sdkMetric.DELTA,
			},
			{
				Name:      "goThreads",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_threads", "goThreads"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "goGoroutines",
				ValueFunc: prometheus.FromValueWithOverriddenName("go_goroutines", "goGoroutines"),
				Type:      sdkMetric.GAUGE,
			},
			// computed
			{
				Name:      "processFdsUtilization",
				ValueFunc: toUtilization(processOpenFds, processMaxFds),
				Type:      sdkMetric.GAUGE,
			},
		},
	},
}

// EtcdQueries are the queries we will do to the control plane
// etcd instances in order to fetch all the raw metrics.
var EtcdQueries = []prometheus.Query{
	{
		MetricName: "etcd_server_has_leader",
	},
	{
		MetricName: "etcd_server_leader_changes_seen_total",
	},
	{
		MetricName: "etcd_mvcc_db_total_size_in_bytes",
	},
	{
		MetricName: "etcd_server_proposals_committed_total",
	},
	{
		MetricName: "etcd_server_proposals_applied_total",
	},
	{
		MetricName: "etcd_server_proposals_pending",
	},
	{
		MetricName: "etcd_server_proposals_failed_total",
	},
	{
		MetricName: "process_open_fds",
	},
	{
		MetricName: "process_max_fds",
	},
	{
		MetricName: "etcd_network_client_grpc_received_bytes_total",
	},
	{
		MetricName: "etcd_network_client_grpc_sent_bytes_total",
	},
	{
		MetricName: "process_resident_memory_bytes",
	},
	{
		MetricName: "process_cpu_seconds_total",
	},
	{
		MetricName: "go_threads",
	},
	{
		MetricName: "go_goroutines",
	},
}

// KSMSpecs are the metric specifications we want to collect from KSM.
var KSMSpecs = definition.SpecGroups{
	"persistentvolume": {
		// kube_persistentvolume_created is marked as an experimental metric, so we instead use the namespace and name
		// labels from kube_persistentvolume_info to create the Entity ID and Entity Type.
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_persistentvolume_info", "persistentvolume"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGeneratorWithCustomGroup("kube_persistentvolume_info", "PersistentVolume"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		MsTypeGuesser:   metricSetTypeGuesserWithCustomGroup("PersistentVolume"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_persistentvolume_created"), Type: sdkMetric.GAUGE},
			{Name: "capacityBytes", ValueFunc: prometheus.FromValue("kube_persistentvolume_capacity_bytes"), Type: sdkMetric.GAUGE},
			{Name: "statusPhase", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "volumeName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "persistentvolume"), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_claim_ref", "name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcNamespace", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_claim_ref", "claim_namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_persistentvolume_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "storageClass", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "storageclass"), Type: sdkMetric.ATTRIBUTE},
			{Name: "hostPath", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "host_path"), Type: sdkMetric.ATTRIBUTE},
			{Name: "hostPathType", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "host_path_type"), Type: sdkMetric.ATTRIBUTE},
			{Name: "localFs", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "local_fs"), Type: sdkMetric.ATTRIBUTE},
			{Name: "localPath", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "local_path"), Type: sdkMetric.ATTRIBUTE},
			{Name: "csiVolumeHandle", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "csi_volume_handle"), Type: sdkMetric.ATTRIBUTE},
			{Name: "csiDriver", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "csi_driver"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nfsPath", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "nfs_path"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nfsServer", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "nfs_server"), Type: sdkMetric.ATTRIBUTE},
			{Name: "iscsiInitiatorName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "iscsi_initiator_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "iscsiLun", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "iscsi_lun"), Type: sdkMetric.ATTRIBUTE},
			{Name: "iscsiIqn", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "iscsi_iqn"), Type: sdkMetric.ATTRIBUTE},
			{Name: "iscsiTargetPortal", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "iscsi_target_portal"), Type: sdkMetric.ATTRIBUTE},
			{Name: "fcTargetWwns", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "fc_target_wwns"), Type: sdkMetric.ATTRIBUTE},
			{Name: "fcLun", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "fc_lun"), Type: sdkMetric.ATTRIBUTE},
			{Name: "fcWwids", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "fc_wwids"), Type: sdkMetric.ATTRIBUTE},
			{Name: "azureDiskName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "azure_disk_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ebsVolumeId", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "ebs_volume_id"), Type: sdkMetric.ATTRIBUTE},
			{Name: "gcePersistentDiskName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolume_info", "gce_persistent_disk_name"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"cronjob": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_cronjob_created", "cronjob"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_cronjob_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_cronjob_created"), Type: sdkMetric.GAUGE},
			{Name: "isActive", ValueFunc: prometheus.FromValue("kube_cronjob_status_active"), Type: sdkMetric.GAUGE},
			{Name: "nextScheduledTime", ValueFunc: prometheus.FromValue("kube_cronjob_next_schedule_time"), Type: sdkMetric.GAUGE},
			{Name: "lastScheduledTime", ValueFunc: prometheus.FromValue("kube_cronjob_status_last_schedule_time"), Type: sdkMetric.GAUGE},
			{Name: "isSuspended", ValueFunc: prometheus.FromValue("kube_cronjob_spec_suspend"), Type: sdkMetric.GAUGE},
			{Name: "specStartingDeadlineSeconds", ValueFunc: prometheus.FromValue("kube_cronjob_spec_starting_deadline_seconds"), Type: sdkMetric.GAUGE},
			{Name: "metadataResourceVersion", ValueFunc: prometheus.FromValue("kube_cronjob_metadata_resource_version"), Type: sdkMetric.GAUGE},
			{Name: "cronjobName", ValueFunc: prometheus.FromLabelValue("kube_cronjob_created", "cronjob"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_cronjob_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_cronjob_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_cronjob_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "schedule", ValueFunc: prometheus.FromLabelValue("kube_cronjob_info", "schedule"), Type: sdkMetric.ATTRIBUTE},
			{Name: "concurrencyPolicy", ValueFunc: prometheus.FromLabelValue("kube_cronjob_info", "concurrency_policy"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"job_name": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_job_created", "job_name"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGeneratorWithCustomGroup("kube_job_created", "job"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		MsTypeGuesser:   metricSetTypeGuesserWithCustomGroup("job"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_job_created"), Type: sdkMetric.GAUGE},
			{Name: "startedAt", ValueFunc: prometheus.FromValue("kube_job_status_start_time"), Type: sdkMetric.GAUGE},
			{Name: "completedAt", ValueFunc: prometheus.FromValue("kube_job_status_completion_time"), Type: sdkMetric.GAUGE},
			{Name: "specParallelism", ValueFunc: prometheus.FromValue("kube_job_spec_parallelism"), Type: sdkMetric.GAUGE},
			{Name: "specCompletions", ValueFunc: prometheus.FromValue("kube_job_spec_completions"), Type: sdkMetric.GAUGE},
			{Name: "specActiveDeadlineSeconds", ValueFunc: prometheus.FromValue("kube_job_spec_active_deadline_seconds"), Type: sdkMetric.GAUGE},
			{Name: "activePods", ValueFunc: prometheus.FromValue("kube_job_status_active"), Type: sdkMetric.GAUGE},
			{Name: "succeededPods", ValueFunc: prometheus.FromValue("kube_job_status_succeeded"), Type: sdkMetric.GAUGE},
			{Name: "failedPods", ValueFunc: prometheus.FromValue("kube_job_status_failed"), Type: sdkMetric.GAUGE},
			{Name: "isComplete", ValueFunc: prometheus.FromLabelValue("kube_job_complete", "condition"), Type: sdkMetric.ATTRIBUTE},
			{Name: "failed", ValueFunc: prometheus.FromLabelValue("kube_job_failed", "condition"), Type: sdkMetric.ATTRIBUTE},
			{Name: "failedPodsReason", ValueFunc: prometheus.FromLabelValue("kube_job_status_failed", "reason"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerName", ValueFunc: prometheus.FromLabelValue("kube_job_owner", "owner_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerKind", ValueFunc: prometheus.FromLabelValue("kube_job_owner", "owner_kind"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerIsController", ValueFunc: prometheus.FromLabelValue("kube_job_owner", "owner_is_controller"), Type: sdkMetric.ATTRIBUTE},
			{Name: "jobName", ValueFunc: prometheus.FromLabelValue("kube_job_created", "job_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_job_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_job_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_job_labels", "label"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"persistentvolumeclaim": {
		// kube_persistentvolumeclaim_created is marked as an experimental metric, so we instead use the namespace and name
		// labels from kube_persistentvolumeclaim_info to create the Entity ID and Entity Type.
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_persistentvolumeclaim_info", "persistentvolumeclaim"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGeneratorWithCustomGroup("kube_persistentvolumeclaim_info", "PersistentVolumeClaim"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		MsTypeGuesser:   metricSetTypeGuesserWithCustomGroup("PersistentVolumeClaim"),
		Specs: []definition.Spec{
			// createdAt is marked as optional because it is an experimental metric and not available in older KSM versions
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_persistentvolumeclaim_created"), Type: sdkMetric.GAUGE},
			{Name: "requestedStorageBytes", ValueFunc: prometheus.FromValue("kube_persistentvolumeclaim_resource_requests_storage_bytes"), Type: sdkMetric.GAUGE},
			{Name: "accessMode", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_access_mode", "access_mode"), Type: sdkMetric.ATTRIBUTE},
			{Name: "statusPhase", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "storageClass", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_info", "storageclass"), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_info", "persistentvolumeclaim"), Type: sdkMetric.ATTRIBUTE},
			{Name: "volumeName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_info", "volumename"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_persistentvolumeclaim_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_persistentvolumeclaim_labels", "label"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"replicaset": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_replicaset_created", "replicaset"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_replicaset_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_replicaset_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_replicaset_spec_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_replicaset_status_ready_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsTotal", ValueFunc: prometheus.FromValue("kube_replicaset_status_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsFullyLabeled", ValueFunc: prometheus.FromValue("kube_replicaset_status_fully_labeled_replicas"), Type: sdkMetric.GAUGE},
			{Name: "observedGeneration", ValueFunc: prometheus.FromValue("kube_replicaset_status_observed_generation"), Type: sdkMetric.GAUGE},
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_replicaset_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "replicasetName", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "replicaset"), Type: sdkMetric.ATTRIBUTE},
			// namespace is here for backwards compatibility, we should use the namespaceName
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "deploymentName", ValueFunc: ksmMetric.GetDeploymentNameForReplicaSet(), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_replicaset_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerName", ValueFunc: prometheus.FromLabelValue("kube_replicaset_owner", "owner_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerKind", ValueFunc: prometheus.FromLabelValue("kube_replicaset_owner", "owner_kind"), Type: sdkMetric.ATTRIBUTE},
			{Name: "ownerIsController", ValueFunc: prometheus.FromLabelValue("kube_replicaset_owner", "owner_is_controller"), Type: sdkMetric.ATTRIBUTE},
			// computed
			{
				Name: "podsMissing", ValueFunc: Subtract(
					definition.Transform(prometheus.FromValue("kube_replicaset_spec_replicas"), fromPrometheusNumeric),
					definition.Transform(prometheus.FromValue("kube_replicaset_status_ready_replicas"), fromPrometheusNumeric)),
				Type: sdkMetric.GAUGE,
			},
		},
	},
	"statefulset": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_statefulset_created", "statefulset"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_statefulset_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_statefulset_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_statefulset_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_statefulset_status_replicas_ready"), Type: sdkMetric.GAUGE},
			{Name: "podsCurrent", ValueFunc: prometheus.FromValue("kube_statefulset_status_replicas_current"), Type: sdkMetric.GAUGE},
			{Name: "podsTotal", ValueFunc: prometheus.FromValue("kube_statefulset_status_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsUpdated", ValueFunc: prometheus.FromValue("kube_statefulset_status_replicas_updated"), Type: sdkMetric.GAUGE},
			{Name: "observedGeneration", ValueFunc: prometheus.FromValue("kube_statefulset_status_observed_generation"), Type: sdkMetric.GAUGE},
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_statefulset_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "currentRevision", ValueFunc: prometheus.FromValue("kube_statefulset_status_current_revision"), Type: sdkMetric.GAUGE},
			{Name: "updateRevision", ValueFunc: prometheus.FromValue("kube_statefulset_status_update_revision"), Type: sdkMetric.GAUGE},
			{Name: "statefulsetName", ValueFunc: prometheus.FromLabelValue("kube_statefulset_created", "statefulset"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_statefulset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_statefulset_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			// computed
			{
				Name: "podsMissing", ValueFunc: Subtract(
					definition.Transform(prometheus.FromValue("kube_statefulset_replicas"), fromPrometheusNumeric),
					definition.Transform(prometheus.FromValue("kube_statefulset_status_replicas_ready"), fromPrometheusNumeric)),
				Type: sdkMetric.GAUGE,
			},
		},
	},
	"daemonset": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_daemonset_created", "daemonset"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_daemonset_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_daemonset_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_daemonset_status_desired_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsScheduled", ValueFunc: prometheus.FromValue("kube_daemonset_status_current_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsAvailable", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_available"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_ready"), Type: sdkMetric.GAUGE},
			{Name: "podsUnavailable", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "podsMisscheduled", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_misscheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsUpdatedScheduled", ValueFunc: prometheus.FromValue("kube_daemonset_status_updated_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "observedGeneration", ValueFunc: prometheus.FromValue("kube_daemonset_status_observed_generation"), Type: sdkMetric.GAUGE},
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_daemonset_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_daemonset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "daemonsetName", ValueFunc: prometheus.FromLabelValue("kube_daemonset_created", "daemonset"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_daemonset_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			// computed
			{
				Name: "podsMissing", ValueFunc: Subtract(
					definition.Transform(prometheus.FromValue("kube_daemonset_status_desired_number_scheduled"), fromPrometheusNumeric),
					definition.Transform(prometheus.FromValue("kube_daemonset_status_number_ready"), fromPrometheusNumeric)),
				Type: sdkMetric.GAUGE,
			},
		},
	},
	"namespace": {
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_namespace_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_namespace_created"), Type: sdkMetric.GAUGE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_namespace_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_namespace_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "status", ValueFunc: prometheus.FromLabelValue("kube_namespace_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_namespace_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "annotation.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_namespace_annotations", "annotation"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"deployment": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_deployment_created", "deployment"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_deployment_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_deployment_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_deployment_spec_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsTotal", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_ready"), Type: sdkMetric.GAUGE},
			{Name: "podsAvailable", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_available"), Type: sdkMetric.GAUGE},
			{Name: "podsUnavailable", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "podsUpdated", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_updated"), Type: sdkMetric.GAUGE},
			{Name: "observedGeneration", ValueFunc: prometheus.FromValue("kube_deployment_status_observed_generation"), Type: sdkMetric.GAUGE},
			{Name: "isPaused", ValueFunc: prometheus.FromValue("kube_deployment_spec_paused"), Type: sdkMetric.GAUGE},
			{Name: "rollingUpdateMaxPodsSurge", ValueFunc: prometheus.FromValue("kube_deployment_spec_strategy_rollingupdate_max_surge"), Type: sdkMetric.GAUGE},
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_deployment_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "conditionAvailable", ValueFunc: prometheus.FromLabelValue("kube_deployment_status_condition_available", "status"), Type: sdkMetric.ATTRIBUTE},
			{Name: "conditionProgressing", ValueFunc: prometheus.FromLabelValue("kube_deployment_status_condition_progressing", "status"), Type: sdkMetric.ATTRIBUTE},
			{Name: "conditionReplicaFailure", ValueFunc: prometheus.FromLabelValue("kube_deployment_status_condition_replica_failure", "status"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podsMaxUnavailable", ValueFunc: prometheus.FromValue("kube_deployment_spec_strategy_rollingupdate_max_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_deployment_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_deployment_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "deploymentName", ValueFunc: prometheus.FromLabelValue("kube_deployment_created", "deployment"), Type: sdkMetric.ATTRIBUTE},
			// Important: The order of these lines is important: we could have the same label in different entities, and we would like to keep the value closer to deployment
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("namespace", "kube_namespace_labels"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_deployment_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "annotation.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_deployment_annotations", "annotation"), Type: sdkMetric.ATTRIBUTE},
			// computed
			{
				Name: "podsMissing", ValueFunc: Subtract(
					definition.Transform(prometheus.FromValue("kube_deployment_spec_replicas"), fromPrometheusNumeric),
					definition.Transform(prometheus.FromValue("kube_deployment_status_replicas"), fromPrometheusNumeric)),
				Type: sdkMetric.GAUGE,
			},
		},
	},
	"service": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_service_created", "service"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_service_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{
				Name:      "createdAt",
				ValueFunc: prometheus.FromValue("kube_service_created"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "namespaceName",
				ValueFunc: prometheus.FromLabelValue("kube_service_created", "namespace"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "serviceName",
				ValueFunc: prometheus.FromLabelValue("kube_service_created", "service"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "loadBalancerIP",
				ValueFunc: prometheus.FromLabelValue("kube_service_info", "load_balancer_ip"),
				Type:      sdkMetric.ATTRIBUTE,
				Optional:  true,
			},
			{
				Name:      "externalName",
				ValueFunc: prometheus.FromLabelValue("kube_service_info", "external_name"),
				Type:      sdkMetric.ATTRIBUTE,
				Optional:  true,
			},
			{
				Name:      "clusterIP",
				ValueFunc: prometheus.FromLabelValue("kube_service_info", "cluster_ip"),
				Type:      sdkMetric.ATTRIBUTE,
				Optional:  true,
			},
			{
				Name:      "label.*",
				ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_service_labels", "label"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "specType",
				ValueFunc: prometheus.FromLabelValue("kube_service_spec_type", "type"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name: "selector.*",
				// Fetched from the APIServer that's why it has the `apiserver` prefix.
				ValueFunc: prometheus.InheritAllSelectorsFrom("service", "apiserver_kube_service_spec_selectors"),
				Type:      sdkMetric.ATTRIBUTE,
			},
		},
	},
	"endpoint": {
		IDGenerator:     prometheus.FromLabelValueEntityIDGenerator("kube_endpoint_created", "endpoint"),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_endpoint_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{
				Name:      "createdAt",
				ValueFunc: prometheus.FromValue("kube_endpoint_created"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "namespaceName",
				ValueFunc: prometheus.FromLabelValue("kube_endpoint_created", "namespace"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "endpointName",
				ValueFunc: prometheus.FromLabelValue("kube_endpoint_created", "endpoint"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "label.*",
				ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_endpoint_labels", "label"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			// KSM < 2.14 - Legacy metrics (pre-aggregated by KSM)
			{
				Name:      "addressAvailable",
				ValueFunc: prometheus.FromValue("kube_endpoint_address_available"),
				Type:      sdkMetric.GAUGE,
				Optional:  true, // Optional: does not exist in KSM >= 2.14
			},
			{
				Name:      "addressNotReady",
				ValueFunc: prometheus.FromValue("kube_endpoint_address_not_ready"),
				Type:      sdkMetric.GAUGE,
				Optional:  true, // Optional: does not exist in KSM >= 2.14
			},
			// KSM >= v2.14 - Detailed metrics (we aggregate by filtering on ready label)
			{
				Name: "addressAvailable",
				ValueFunc: prometheus.CountFromValueWithLabelsFilter(
					"kube_endpoint_address",
					"addressAvailable",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"ready": "true",
					}),
				),
				Type:     sdkMetric.GAUGE,
				Optional: true, // Optional: may not exist in KSM < 2.14
			},
			{
				Name: "addressNotReady",
				ValueFunc: prometheus.CountFromValueWithLabelsFilter(
					"kube_endpoint_address",
					"addressNotReady",
					prometheus.IncludeOnlyWhenLabelMatchFilter(map[string]string{
						"ready": "false",
					}),
				),
				Type:     sdkMetric.GAUGE,
				Optional: true, // Optional: may not exist in KSM < 2.14
			},
		},
	},
	// We get Pod metrics from kube-state-metrics for those pods that are in
	// "Pending" status and are not scheduled. We can't get the data from Kubelet because
	// they aren't running in any node and the information about them is only
	// present in the API.
	"pod": {
		IDGenerator:     prometheus.FromLabelsValueEntityIDGeneratorForPendingPods(),
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_pod_status_phase"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_pod_created"), Type: sdkMetric.GAUGE},
			{Name: "createdKind", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "created_by_kind"), Type: sdkMetric.ATTRIBUTE},
			{Name: "createdBy", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "created_by_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeIP", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "host_ip"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "node"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "pod"), Type: sdkMetric.ATTRIBUTE},
			// we are adding as default `false` since all ksm pods used refers to pending pods due to the IDGenerator.
			{Name: "isReady", ValueFunc: definition.Transform(fetchWithDefault(prometheus.FromLabelValue("kube_pod_status_ready", "condition"), "false"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "status", ValueFunc: prometheus.FromLabelValue("kube_pod_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isScheduled", ValueFunc: definition.Transform(prometheus.FromLabelValue("kube_pod_status_scheduled", "condition"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "deploymentName", ValueFunc: ksmMetric.GetDeploymentNameForPod(), Type: sdkMetric.ATTRIBUTE},
			{Name: "priorityClassName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "priority_class"), Type: sdkMetric.ATTRIBUTE, Optional: true},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_pod_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			{Name: "annotation.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_pod_annotations", "annotation"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"horizontalpodautoscaler": {
		IDGenerator: prometheus.FromLabelValueEntityIDGenerator("kube_horizontalpodautoscaler_status_current_replicas", "horizontalpodautoscaler"),
		// group customized for backwards compatibility reasons (Metrics where renamed in KSM v2)
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGeneratorWithCustomGroup("kube_horizontalpodautoscaler_status_current_replicas", "hpa"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		MsTypeGuesser:   metricSetTypeGuesserWithCustomGroup("hpa"), // group customized for backwards compatibility reasons
		Specs: []definition.Spec{
			// The generation observed by the HorizontalPodAutoscaler controller. not sure if interesting to get
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "maxReplicas", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_spec_max_replicas"), Type: sdkMetric.GAUGE},
			{Name: "minReplicas", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_spec_min_replicas"), Type: sdkMetric.GAUGE},
			// TODO this metric has a couple of dimensions (metric_name, target_type) that might be useful to add
			{Name: "targetMetric", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_spec_target_metric"), Type: sdkMetric.GAUGE},
			{Name: "currentReplicas", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_status_current_replicas"), Type: sdkMetric.GAUGE},
			{Name: "desiredReplicas", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_status_desired_replicas"), Type: sdkMetric.GAUGE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_horizontalpodautoscaler_metadata_generation", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_horizontalpodautoscaler_labels", "label"), Type: sdkMetric.ATTRIBUTE},
			// TODO: is* metrics will be either true or `NULL`, but never false if the condition is not reported. This is not ideal.
			{Name: "isActive", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_status_condition_active")},
			{Name: "isAble", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_status_condition_able")},
			{Name: "isLimited", ValueFunc: prometheus.FromValue("kube_horizontalpodautoscaler_status_condition_limited")},
		},
	},
	"resourcequota": {
		TypeGenerator:   prometheus.FromLabelValueEntityTypeGenerator("kube_resourcequota_created"),
		NamespaceGetter: prometheus.FromLabelGetNamespace,
		SplitByLabel:    "resource",
		SliceMetricName: "kube_resourcequota",
		Specs: []definition.Spec{
			{
				Name:      "createdAt",
				ValueFunc: prometheus.FromValue("kube_resourcequota_created"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name: "namespaceName",
				ValueFunc: prometheus.FromLabelValue(
					"kube_resourcequota_created",
					"namespace",
				),
				Type: sdkMetric.ATTRIBUTE,
			},
			{
				Name: "resourcequotaName", // This will be the name of your new column.
				ValueFunc: prometheus.FromLabelValue(
					"kube_resourcequota_created", // The stable source metric.
					"resourcequota",              // The label to extract the value from.
				),
				Type: sdkMetric.ATTRIBUTE,
			},
			{
				Name: "resource", // This will be the name of your new column.
				ValueFunc: prometheus.FromLabelValue(
					"kube_resourcequota", // The stable source metric.
					"resource",           // The label to extract the value from.
				),
				Type: sdkMetric.ATTRIBUTE,
			},
			{
				Name: "resource.*",
				// This single entry uses our new generic function to create the 'resource' attribute
				// and the 'hard' and 'used' metrics for each sub-entity.
				ValueFunc: prometheus.FromFlattenedMetrics(
					"kube_resourcequota",
					"type",
				),
				Type: sdkMetric.GAUGE,
			},
			{
				Name: "label.*",
				// This uses a generic function to fetch all Kubernetes labels.
				ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_resourcequota_labels", "label"),
				Type:      sdkMetric.ATTRIBUTE,
				Optional:  true,
			},
			{
				Name: "annotation.*",
				// This uses a generic function to fetch all Kubernetes annotations.
				ValueFunc: prometheus.FromMetricWithPrefixedLabels("kube_resourcequota_annotations", "annotation"),
				Type:      sdkMetric.ATTRIBUTE,
				Optional:  true,
			},
		},
	},
}

// KSMQueries are the queries we will do to KSM in order to fetch all the raw metrics.
var KSMQueries = []prometheus.Query{
	// kube_persistentvolume_created is an EXPERIMENTAL KSM metric
	{MetricName: "kube_persistentvolume_created"},
	{MetricName: "kube_persistentvolume_capacity_bytes"},
	{MetricName: "kube_persistentvolume_status_phase", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_persistentvolume_status_phase{persistentvolume="e2e-resources",phase="Pending"} 0
		// kube_persistentvolume_status_phase{persistentvolume="e2e-resources",phase="Available"} 1
		// kube_persistentvolume_status_phase{persistentvolume="e2e-resources",phase="Bound"} 0
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_persistentvolume_claim_ref"},
	{MetricName: "kube_persistentvolume_info"},
	{MetricName: "kube_persistentvolume_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_cronjob_info"},
	{MetricName: "kube_cronjob_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_cronjob_created"},
	{MetricName: "kube_cronjob_next_schedule_time"},
	{MetricName: "kube_cronjob_status_active"},
	{MetricName: "kube_cronjob_status_last_schedule_time"},
	{MetricName: "kube_cronjob_spec_suspend"},
	{MetricName: "kube_cronjob_spec_starting_deadline_seconds"},
	{MetricName: "kube_cronjob_metadata_resource_version"},
	{MetricName: "kube_job_info"},
	{MetricName: "kube_job_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_job_owner"},
	{MetricName: "kube_job_spec_parallelism"},
	{MetricName: "kube_job_spec_completions"},
	{MetricName: "kube_job_spec_active_deadline_seconds"},
	{MetricName: "kube_job_status_active"},
	{MetricName: "kube_job_status_succeeded"},
	{MetricName: "kube_job_status_failed", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_job_status_failed{namespace="default",job_name="e2e-resources-failjob",reason="BackoffLimitExceeded"} 1
		// kube_job_status_failed{namespace="default",job_name="e2e-resources-failjob",reason="DeadLineExceeded"} 0
		// kube_job_status_failed{namespace="default",job_name="e2e-resources-failjob",reason="Evicted"} 0
		// kube_job_status_failed{namespace="default",job_name="e2e-resources-cronjob-27931661"} 0
		//
		// KSM should never produce a positive value for more than one status, so we can simply fetch
		// only values which has value 1 for processing.
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_job_status_start_time"},
	{MetricName: "kube_job_status_completion_time"},
	{MetricName: "kube_job_complete", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_job_complete{namespace="default",job_name="e2e-resources-cronjob",condition="true"} 1
		// kube_job_complete{namespace="default",job_name="e2e-resources-cronjob",condition="false"} 0
		// kube_job_complete{namespace="default",job_name="e2e-resources-cronjob",condition="unknown"} 0
		//
		// KSM should never produce a positive value for more than one status, so we can simply fetch
		// only values which has value 1 for processing.
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_job_failed", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_job_failed{namespace="default",job_name="e2e-resources-failjob",condition="true"} 1
		// kube_job_failed{namespace="default",job_name="e2e-resources-failjob",condition="false"} 0
		// kube_job_failed{namespace="default",job_name="e2e-resources-failjob",condition="unknown"} 0
		//
		// KSM should never produce a positive value for more than one status, so we can simply fetch
		// only values which has value 1 for processing.
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_job_created"},
	// kube_persistentvolumeclaim_created is an EXPERIMENTAL KSM metric
	{MetricName: "kube_persistentvolumeclaim_created"},
	{MetricName: "kube_persistentvolumeclaim_access_mode"},
	{MetricName: "kube_persistentvolumeclaim_info"},
	{MetricName: "kube_persistentvolumeclaim_resource_requests_storage_bytes"},
	{MetricName: "kube_persistentvolumeclaim_status_phase", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_persistentvolumeclaim_status_phase{namespace="default",persistentvolumeclaim="e2e-resources",phase="Lost"} 0
		// kube_persistentvolumeclaim_status_phase{namespace="default",persistentvolumeclaim="e2e-resources",phase="Bound"} 1
		// kube_persistentvolumeclaim_status_phase{namespace="default",persistentvolumeclaim="e2e-resources",phase="Pending"} 0
		//
		// KSM should never produce a positive value for more than one status, so we can simply fetch
		// only values which has value 1 for processing.
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_persistentvolumeclaim_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_statefulset_replicas"},
	{MetricName: "kube_statefulset_status_replicas_ready"},
	{MetricName: "kube_statefulset_status_replicas"},
	{MetricName: "kube_statefulset_status_replicas_current"},
	{MetricName: "kube_statefulset_status_replicas_updated"},
	{MetricName: "kube_statefulset_status_observed_generation"},
	{MetricName: "kube_statefulset_metadata_generation"},
	{MetricName: "kube_statefulset_status_current_revision"},
	{MetricName: "kube_statefulset_status_update_revision"},
	{MetricName: "kube_statefulset_created"},
	{MetricName: "kube_statefulset_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_daemonset_created"},
	{MetricName: "kube_daemonset_status_desired_number_scheduled"},
	{MetricName: "kube_daemonset_status_current_number_scheduled"},
	{MetricName: "kube_daemonset_status_number_ready"},
	{MetricName: "kube_daemonset_status_number_available"},
	{MetricName: "kube_daemonset_status_number_unavailable"},
	{MetricName: "kube_daemonset_status_number_misscheduled"},
	{MetricName: "kube_daemonset_status_updated_number_scheduled"},
	{MetricName: "kube_daemonset_status_observed_generation"},
	{MetricName: "kube_daemonset_metadata_generation"},
	{MetricName: "kube_daemonset_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_replicaset_spec_replicas"},
	{MetricName: "kube_replicaset_status_ready_replicas"},
	{MetricName: "kube_replicaset_status_replicas"},
	{MetricName: "kube_replicaset_status_fully_labeled_replicas"},
	{MetricName: "kube_replicaset_status_observed_generation"},
	{MetricName: "kube_replicaset_metadata_generation"},
	{MetricName: "kube_replicaset_created"},
	{MetricName: "kube_replicaset_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_replicaset_owner"},
	{MetricName: "kube_namespace_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_namespace_annotations"},
	{MetricName: "kube_namespace_created"},
	{MetricName: "kube_namespace_status_phase", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_deployment_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_deployment_annotations"},
	{MetricName: "kube_deployment_created"},
	{MetricName: "kube_deployment_spec_replicas"},
	{MetricName: "kube_deployment_status_replicas"},
	{MetricName: "kube_deployment_status_replicas_ready"},
	{MetricName: "kube_deployment_status_replicas_available"},
	{MetricName: "kube_deployment_status_replicas_unavailable"},
	{MetricName: "kube_deployment_status_replicas_updated"},
	{MetricName: "kube_deployment_status_observed_generation"},
	{MetricName: "kube_deployment_spec_paused"},
	{MetricName: "kube_deployment_spec_strategy_rollingupdate_max_surge"},
	{MetricName: "kube_deployment_metadata_generation"},
	{
		MetricName: "kube_deployment_status_condition",
		CustomName: "kube_deployment_status_condition_available",
		Labels: prometheus.QueryLabels{
			Labels: prometheus.Labels{"condition": "Available"},
		},
		Value: prometheus.QueryValue{
			Value: prometheus.GaugeValue(1),
		},
	},
	{
		MetricName: "kube_deployment_status_condition",
		CustomName: "kube_deployment_status_condition_progressing",
		Labels: prometheus.QueryLabels{
			Labels: prometheus.Labels{"condition": "Progressing"},
		},
		Value: prometheus.QueryValue{
			Value: prometheus.GaugeValue(1),
		},
	},
	{
		MetricName: "kube_deployment_status_condition",
		CustomName: "kube_deployment_status_condition_replica_failure",
		Labels: prometheus.QueryLabels{
			Labels: prometheus.Labels{"condition": "ReplicaFailure"},
		},
		Value: prometheus.QueryValue{
			Value: prometheus.GaugeValue(1),
		},
	},
	{MetricName: "kube_deployment_spec_strategy_rollingupdate_max_unavailable"},
	{MetricName: "kube_pod_status_phase", Labels: prometheus.QueryLabels{
		Labels: prometheus.Labels{"phase": "Pending"},
	}, Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_info"},
	{MetricName: "kube_pod_created"},
	{MetricName: "kube_pod_labels"},
	{MetricName: "kube_pod_annotations"},
	{MetricName: "kube_pod_status_scheduled", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_status_ready", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_start_time"},
	{MetricName: "kube_pod_spec_priority_class"},
	{MetricName: "kube_service_created"},
	{MetricName: "kube_service_labels"},
	{MetricName: "kube_service_info"},
	{MetricName: "kube_service_status_load_balancer_ingress"},
	{MetricName: "kube_service_spec_type", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_endpoint_created"},
	{MetricName: "kube_endpoint_labels"},
	{MetricName: "kube_endpoint_address_not_ready"},
	{MetricName: "kube_endpoint_address_available"},
	{MetricName: "kube_endpoint_address"},
	// hpa
	{MetricName: "kube_horizontalpodautoscaler_info"},
	{MetricName: "kube_horizontalpodautoscaler_labels"},
	{MetricName: "kube_horizontalpodautoscaler_metadata_generation"},
	{MetricName: "kube_horizontalpodautoscaler_spec_max_replicas"},
	{MetricName: "kube_horizontalpodautoscaler_spec_min_replicas"},
	{MetricName: "kube_horizontalpodautoscaler_spec_target_metric"},
	{
		MetricName: "kube_horizontalpodautoscaler_status_condition", CustomName: "kube_horizontalpodautoscaler_status_condition_active",
		Labels: prometheus.QueryLabels{
			Labels:   prometheus.Labels{"condition": "ScalingActive", "status": "true"},
			Operator: prometheus.QueryOpAnd,
		},
	},
	{
		MetricName: "kube_horizontalpodautoscaler_status_condition", CustomName: "kube_horizontalpodautoscaler_status_condition_able",
		Labels: prometheus.QueryLabels{
			Labels:   prometheus.Labels{"condition": "AbleToScale", "status": "true"},
			Operator: prometheus.QueryOpAnd,
		},
	},
	{
		MetricName: "kube_horizontalpodautoscaler_status_condition", CustomName: "kube_horizontalpodautoscaler_status_condition_limited",
		Labels: prometheus.QueryLabels{
			Labels:   prometheus.Labels{"condition": "ScalingLimited", "status": "true"},
			Operator: prometheus.QueryOpAnd,
		},
	},
	{MetricName: "kube_horizontalpodautoscaler_status_current_replicas"},
	{MetricName: "kube_horizontalpodautoscaler_status_desired_replicas"},
	// Node info and labels, so sample containing conditions also has this information.
	{MetricName: "kube_node_info"},
	{MetricName: "kube_node_labels"},
	// Node status condition
	{MetricName: "kube_node_status_condition", Value: prometheus.QueryValue{
		// Since we aggregate metrics which look like the following:
		//
		// kube_node_status_condition{node="minikube",condition="MemoryPressure",status="true"} 0
		// kube_node_status_condition{node="minikube",condition="MemoryPressure",status="false"} 1
		// kube_node_status_condition{node="minikube",condition="MemoryPressure",status="unknown"} 0
		//
		// KSM should never produce a positive value for more than one status, so we can simply fetch
		// only values which has value 1 for processing.
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_node_spec_unschedulable"},
	{MetricName: "kube_resourcequota"},
	{MetricName: "kube_resourcequota_created"},
	{MetricName: "kube_resourcequota_labels"},
	{MetricName: "kube_resourcequota_annotations"},
}

// CadvisorQueries are the queries we will do to the kubelet metrics cadvisor endpoint in order to fetch all the raw metrics.
var CadvisorQueries = []prometheus.Query{
	{
		MetricName: "container_memory_usage_bytes",
		Labels: prometheus.QueryLabels{
			Operator: prometheus.QueryOpNor,
			Labels: prometheus.Labels{
				"container_name": "",
			},
		},
	},
	{MetricName: "container_cpu_cfs_periods_total"},
	{MetricName: "container_cpu_cfs_throttled_periods_total"},
	{MetricName: "container_cpu_cfs_throttled_seconds_total"},
	{MetricName: "container_memory_mapped_file"},
	{MetricName: "container_oom_events_total"},
}

// NewKubeletSpecs creates the metric specifications we want to collect from Kubelet.
// It accepts an optional interface cache for network metric optimization.
//
//nolint:funlen // Large spec definition is acceptable - it's configuration, not logic
func NewKubeletSpecs(interfaceCache *kubeletMetric.InterfaceCache) definition.SpecGroups {
	return definition.SpecGroups{
		"pod": {
			IDGenerator:     kubeletMetric.FromRawEntityIDGroupEntityIDGenerator("namespace"),
			TypeGenerator:   kubeletMetric.FromRawGroupsEntityTypeGenerator,
			NamespaceGetter: kubeletMetric.FromLabelGetNamespace,
			Specs: []definition.Spec{
				// /stats/summary endpoint
				{Name: "net.rxBytesPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("rxBytes", interfaceCache), Type: sdkMetric.RATE},
				{Name: "net.txBytesPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("txBytes", interfaceCache), Type: sdkMetric.RATE},
				{Name: "net.errorsPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("errors", interfaceCache), Type: sdkMetric.RATE},

				// /pods endpoint
				{Name: "createdAt", ValueFunc: definition.Transform(definition.FromRaw("createdAt"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "scheduledAt", ValueFunc: definition.Transform(definition.FromRaw("scheduledAt"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "initializedAt", ValueFunc: definition.Transform(definition.FromRaw("initializedAt"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "containersReadyAt", ValueFunc: definition.Transform(definition.FromRaw("containersReadyAt"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "readyAt", ValueFunc: definition.Transform(definition.FromRaw("readyAt"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "startTime", ValueFunc: definition.Transform(definition.FromRaw("startTime"), toTimestamp), Type: sdkMetric.GAUGE},
				{Name: "createdKind", ValueFunc: definition.FromRaw("createdKind"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "createdBy", ValueFunc: definition.FromRaw("createdBy"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "nodeIP", ValueFunc: definition.FromRaw("nodeIP"), Type: sdkMetric.ATTRIBUTE},
				{Name: "podIP", ValueFunc: definition.FromRaw("podIP"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "isReady", ValueFunc: definition.Transform(definition.FromRaw("isReady"), toNumericBoolean), Type: sdkMetric.GAUGE},
				{Name: "status", ValueFunc: definition.FromRaw("status"), Type: sdkMetric.ATTRIBUTE},
				{Name: "isScheduled", ValueFunc: definition.Transform(definition.FromRaw("isScheduled"), toNumericBoolean), Type: sdkMetric.GAUGE},
				{Name: "deploymentName", ValueFunc: definition.FromRaw("deploymentName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "daemonsetName", ValueFunc: definition.FromRaw("daemonsetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "jobName", ValueFunc: definition.FromRaw("jobName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "replicasetName", ValueFunc: definition.FromRaw("replicasetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "statefulsetName", ValueFunc: definition.FromRaw("statefulsetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "priority", ValueFunc: definition.FromRaw("priority"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "priorityClassName", ValueFunc: definition.FromRaw("priorityClassName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "label.*", ValueFunc: definition.Transform(definition.FromRaw("labels"), kubeletMetric.OneMetricPerLabel), Type: sdkMetric.ATTRIBUTE},
				{Name: "reason", ValueFunc: definition.FromRaw("reason"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "message", ValueFunc: definition.FromRaw("message"), Type: sdkMetric.ATTRIBUTE, Optional: true},
			},
		},
		"container": {
			IDGenerator:     kubeletMetric.FromRawGroupsEntityIDGenerator("containerName"),
			TypeGenerator:   kubeletMetric.FromRawGroupsEntityTypeGenerator,
			NamespaceGetter: kubeletMetric.FromLabelGetNamespace,
			Specs: []definition.Spec{
				// /stats/summary endpoint
				{Name: "memoryUsedBytes", ValueFunc: definition.FromRaw("usageBytes"), Type: sdkMetric.GAUGE},
				{Name: "memoryWorkingSetBytes", ValueFunc: workingSetBytes, Type: sdkMetric.GAUGE},
				{Name: "cpuUsedCores", ValueFunc: _cpuUsedCores, Type: sdkMetric.GAUGE},
				{Name: "fsAvailableBytes", ValueFunc: definition.FromRaw("fsAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsCapacityBytes", ValueFunc: definition.FromRaw("fsCapacityBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsUsedBytes", ValueFunc: definition.FromRaw("fsUsedBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsUsedPercent", ValueFunc: toComplementPercentage("fsUsedBytes", "fsAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesFree", ValueFunc: definition.FromRaw("fsInodesFree"), Type: sdkMetric.GAUGE},
				{Name: "fsInodes", ValueFunc: definition.FromRaw("fsInodes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesUsed", ValueFunc: definition.FromRaw("fsInodesUsed"), Type: sdkMetric.GAUGE},

				// /metrics/cadvisor endpoint
				{Name: "containerID", ValueFunc: definition.FromRaw("containerID"), Type: sdkMetric.ATTRIBUTE},
				{Name: "containerImageID", ValueFunc: definition.FromRaw("containerImageID"), Type: sdkMetric.ATTRIBUTE},
				{Name: "containerMemoryMappedFileBytes", ValueFunc: definition.FromRaw("container_memory_mapped_file"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "containerOOMEventsDelta", ValueFunc: definition.FromRaw("container_oom_events_total"), Type: sdkMetric.PDELTA, Optional: true},
				// In openshift (and possibly in other environments) these metrics were missing at first for pods that were not throttled.
				{Name: "containerCpuCfsPeriodsDelta", ValueFunc: definition.FromRaw("container_cpu_cfs_periods_total"), Type: sdkMetric.DELTA, Optional: true},
				{Name: "containerCpuCfsThrottledPeriodsDelta", ValueFunc: definition.FromRaw("container_cpu_cfs_throttled_periods_total"), Type: sdkMetric.DELTA, Optional: true},
				{Name: "containerCpuCfsThrottledSecondsDelta", ValueFunc: definition.FromRaw("container_cpu_cfs_throttled_seconds_total"), Type: sdkMetric.DELTA, Optional: true},
				{Name: "containerCpuCfsPeriodsTotal", ValueFunc: definition.FromRaw("container_cpu_cfs_periods_total"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "containerCpuCfsThrottledPeriodsTotal", ValueFunc: definition.FromRaw("container_cpu_cfs_throttled_periods_total"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "containerCpuCfsThrottledSecondsTotal", ValueFunc: definition.FromRaw("container_cpu_cfs_throttled_seconds_total"), Type: sdkMetric.GAUGE, Optional: true},

				// /pods endpoint
				{Name: "containerName", ValueFunc: definition.FromRaw("containerName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "containerImage", ValueFunc: definition.FromRaw("containerImage"), Type: sdkMetric.ATTRIBUTE},
				{Name: "deploymentName", ValueFunc: definition.FromRaw("deploymentName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "daemonsetName", ValueFunc: definition.FromRaw("daemonsetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "jobName", ValueFunc: definition.FromRaw("jobName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "replicasetName", ValueFunc: definition.FromRaw("replicasetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "statefulsetName", ValueFunc: definition.FromRaw("statefulsetName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "nodeIP", ValueFunc: definition.FromRaw("nodeIP"), Type: sdkMetric.ATTRIBUTE},
				{Name: "restartCount", ValueFunc: definition.FromRaw("restartCount"), Type: sdkMetric.GAUGE},
				{Name: "restartCountDelta", ValueFunc: definition.FromRaw("restartCount"), Type: sdkMetric.PDELTA},
				{Name: "cpuRequestedCores", ValueFunc: cpuRequestedCores, Type: sdkMetric.GAUGE, Optional: true},
				{Name: "cpuLimitCores", ValueFunc: cpuLimitCores, Type: sdkMetric.GAUGE, Optional: true},
				{Name: "memoryRequestedBytes", ValueFunc: definition.FromRaw("memoryRequestedBytes"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "memoryLimitBytes", ValueFunc: definition.FromRaw("memoryLimitBytes"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "status", ValueFunc: definition.FromRaw("status"), Type: sdkMetric.ATTRIBUTE},
				{Name: "isReady", ValueFunc: definition.Transform(definition.FromRaw("isReady"), toNumericBoolean), Type: sdkMetric.GAUGE},
				{Name: "reason", ValueFunc: definition.FromRaw("reason"), Type: sdkMetric.ATTRIBUTE, Optional: true}, // Previously called statusWaitingReason
				{Name: "lastTerminatedExitCode", ValueFunc: definition.FromRaw("lastTerminatedExitCode"), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "lastTerminatedExitReason", ValueFunc: definition.FromRaw("lastTerminatedExitReason"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "lastTerminatedTimestamp", ValueFunc: definition.Transform(definition.FromRaw("lastTerminatedTimestamp"), toTimestamp), Type: sdkMetric.GAUGE, Optional: true},

				// Inherit from pod
				{Name: "label.*", ValueFunc: definition.Transform(definition.FromRaw("labels"), kubeletMetric.OneMetricPerLabel), Type: sdkMetric.ATTRIBUTE},

				// computed
				{Name: "cpuCoresUtilization", ValueFunc: toUtilization(_cpuUsedCores, cpuLimitCores), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "requestedCpuCoresUtilization", ValueFunc: toUtilization(_cpuUsedCores, cpuRequestedCores), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "memoryUtilization", ValueFunc: toUtilization(definition.FromRaw("usageBytes"), definition.FromRaw("memoryLimitBytes")), Type: sdkMetric.GAUGE, Optional: true},
				{Name: "requestedMemoryUtilization", ValueFunc: toUtilization(definition.FromRaw("usageBytes"), definition.FromRaw("memoryRequestedBytes")), Type: sdkMetric.GAUGE, Optional: true},
			},
		},
		"node": {
			TypeGenerator: kubeletMetric.FromRawGroupsEntityTypeGenerator,
			Specs: []definition.Spec{
				{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "cpuUsedCores", ValueFunc: _cpuUsedCores, Type: sdkMetric.GAUGE},
				{Name: "cpuUsedCoreMilliseconds", ValueFunc: definition.Transform(definition.FromRaw("usageCoreNanoSeconds"), fromNanoToMilli), Type: sdkMetric.GAUGE},
				{Name: "memoryUsedBytes", ValueFunc: definition.FromRaw("memoryUsageBytes"), Type: sdkMetric.GAUGE},
				{Name: "memoryAvailableBytes", ValueFunc: definition.FromRaw("memoryAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "memoryWorkingSetBytes", ValueFunc: definition.FromRaw("memoryWorkingSetBytes"), Type: sdkMetric.GAUGE},
				{Name: "memoryRssBytes", ValueFunc: definition.FromRaw("memoryRssBytes"), Type: sdkMetric.GAUGE},
				{Name: "memoryPageFaults", ValueFunc: definition.FromRaw("memoryPageFaults"), Type: sdkMetric.GAUGE},
				{Name: "memoryMajorPageFaultsPerSecond", ValueFunc: definition.FromRaw("memoryMajorPageFaults"), Type: sdkMetric.RATE},
				{Name: "net.rxBytesPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("rxBytes", interfaceCache), Type: sdkMetric.RATE},
				{Name: "net.txBytesPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("txBytes", interfaceCache), Type: sdkMetric.RATE},
				{Name: "net.errorsPerSecond", ValueFunc: kubeletMetric.FromRawWithFallbackToDefaultInterface("errors", interfaceCache), Type: sdkMetric.RATE},
				{Name: "fsAvailableBytes", ValueFunc: definition.FromRaw("fsAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsCapacityBytes", ValueFunc: definition.FromRaw("fsCapacityBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsUsedBytes", ValueFunc: definition.FromRaw("fsUsedBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesFree", ValueFunc: definition.FromRaw("fsInodesFree"), Type: sdkMetric.GAUGE},
				{Name: "fsInodes", ValueFunc: definition.FromRaw("fsInodes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesUsed", ValueFunc: definition.FromRaw("fsInodesUsed"), Type: sdkMetric.GAUGE},
				{Name: "runtimeAvailableBytes", ValueFunc: definition.FromRaw("runtimeAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "runtimeCapacityBytes", ValueFunc: definition.FromRaw("runtimeCapacityBytes"), Type: sdkMetric.GAUGE},
				{Name: "runtimeUsedBytes", ValueFunc: definition.FromRaw("runtimeUsedBytes"), Type: sdkMetric.GAUGE},
				{Name: "runtimeInodesFree", ValueFunc: definition.FromRaw("runtimeInodesFree"), Type: sdkMetric.GAUGE},
				{Name: "runtimeInodes", ValueFunc: definition.FromRaw("runtimeInodes"), Type: sdkMetric.GAUGE},
				{Name: "runtimeInodesUsed", ValueFunc: definition.FromRaw("runtimeInodesUsed"), Type: sdkMetric.GAUGE},
				{Name: "label.*", ValueFunc: definition.Transform(definition.FromRaw("labels"), kubeletMetric.OneMetricPerLabel), Type: sdkMetric.ATTRIBUTE},
				{Name: "allocatable.*", ValueFunc: definition.Transform(definition.FromRaw("allocatable"), kubeletMetric.OneAttributePerAllocatable), Type: sdkMetric.GAUGE},
				{Name: "capacity.*", ValueFunc: definition.Transform(definition.FromRaw("capacity"), kubeletMetric.OneAttributePerCapacity), Type: sdkMetric.GAUGE},
				{Name: "condition.*", ValueFunc: definition.Transform(definition.FromRaw("conditions"), kubeletMetric.PrefixFromMapInt("condition.")), Type: sdkMetric.GAUGE},
				{Name: "unschedulable", ValueFunc: definition.Transform(definition.FromRaw("unschedulable"), toNumericBoolean), Type: sdkMetric.GAUGE},
				{Name: "memoryRequestedBytes", ValueFunc: definition.FromRaw("memoryRequestedBytes"), Type: sdkMetric.GAUGE},
				{Name: "cpuRequestedCores", ValueFunc: cpuRequestedCores, Type: sdkMetric.GAUGE},
				{Name: "kubeletVersion", ValueFunc: definition.FromRaw("kubeletVersion"), Type: sdkMetric.ATTRIBUTE},
				{Name: "runningPods", ValueFunc: definition.FromRaw("runningPods"), Type: sdkMetric.GAUGE},
				// computed
				{Name: "fsCapacityUtilization", ValueFunc: toUtilization(definition.FromRaw("fsUsedBytes"), definition.FromRaw("fsCapacityBytes")), Type: sdkMetric.GAUGE},
				{Name: "allocatableCpuCoresUtilization", ValueFunc: toUtilization(_cpuUsedCores, definition.FromRaw("allocatableCpuCores")), Type: sdkMetric.GAUGE},
				{Name: "allocatableMemoryUtilization", ValueFunc: toUtilization(workingSetBytes, definition.FromRaw("allocatableMemoryBytes")), Type: sdkMetric.GAUGE},
			},
		},
		"volume": {
			TypeGenerator:   kubeletMetric.FromRawGroupsEntityTypeGenerator,
			NamespaceGetter: kubeletMetric.FromLabelGetNamespace,
			Specs: []definition.Spec{
				{Name: "volumeName", ValueFunc: definition.FromRaw("volumeName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
				{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
				{Name: "persistent", ValueFunc: isPersistentVolume(), Type: sdkMetric.ATTRIBUTE},
				{Name: "pvcName", ValueFunc: definition.FromRaw("pvcName"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "pvcNamespace", ValueFunc: definition.FromRaw("pvcNamespace"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "pvcNamespaceName", ValueFunc: definition.FromRaw("pvcNamespace"), Type: sdkMetric.ATTRIBUTE, Optional: true},
				{Name: "fsAvailableBytes", ValueFunc: definition.FromRaw("fsAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsCapacityBytes", ValueFunc: definition.FromRaw("fsCapacityBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsUsedBytes", ValueFunc: definition.FromRaw("fsUsedBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsUsedPercent", ValueFunc: toComplementPercentage("fsUsedBytes", "fsAvailableBytes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesFree", ValueFunc: definition.FromRaw("fsInodesFree"), Type: sdkMetric.GAUGE},
				{Name: "fsInodes", ValueFunc: definition.FromRaw("fsInodes"), Type: sdkMetric.GAUGE},
				{Name: "fsInodesUsed", ValueFunc: definition.FromRaw("fsInodesUsed"), Type: sdkMetric.GAUGE},
			},
		},
	}
}

// KubeletSpecs is the default metric specifications for Kubelet with no interface cache.
//
//nolint:gochecknoglobals // Backward compatibility - used by tests and static tooling
var KubeletSpecs = NewKubeletSpecs(nil)

func isPersistentVolume() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		name, err := definition.FromRaw("pvcName")(groupLabel, entityID, groups)
		if err == nil && name != "" {
			return "true", nil
		}
		return "false", nil
	}
}

func computePercentage(dividend, divisor interface{}) (definition.FetchedValue, error) {
	var a, b float64

	a, err := convertValue(dividend)
	if err != nil {
		return nil, fmt.Errorf("casting dividend: %w", err)
	}

	b, err = convertValue(divisor)
	if err != nil {
		return nil, fmt.Errorf("casting divisor: %w", err)
	}

	if b == float64(0) {
		return nil, fmt.Errorf("division by zero")
	}

	return a / b * 100, nil
}

func convertValue(v interface{}) (float64, error) {
	switch v := v.(type) {
	case uint:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case prometheus.GaugeValue:
		return float64(v), nil
	case float64:
		return v, nil
	case definition.FetchedValues:
		if len(v) != 1 {
			return 0, fmt.Errorf("unable to convert FetchedValues")
		}
		for _, k := range v {
			return convertValue(k)
		}
		return 0, fmt.Errorf("unable to convert FetchedValues")
	default:
		return 0, fmt.Errorf("type not supported %T", v)
	}
}

func toComplementPercentage(desiredMetric, complementMetric string) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		complement, err := definition.FromRaw(complementMetric)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		desired, err := definition.FromRaw(desiredMetric)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		v, err := computePercentage(desired.(uint64), desired.(uint64)+complement.(uint64))
		if err != nil {
			return nil, fmt.Errorf("error computing percentage for %s & %s: %s", desiredMetric, complementMetric, err)
		}

		return v, nil
	}
}

func toUtilization(dividendFunc, divisorFunc definition.FetchFunc) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		dividend, err := dividendFunc(groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("getting divident metric: %w", err)
		}

		divisor, err := divisorFunc(groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("getting divisor metric: %w", err)
		}

		value, err := computePercentage(dividend, divisor)
		if err != nil {
			return nil, fmt.Errorf("computing utilization: %w", err)
		}

		return value, nil
	}
}

// Used to transform from usageNanoCores to cpuUsedCores
func fromNano(value definition.FetchedValue) (definition.FetchedValue, error) {
	v, ok := value.(uint64)
	if !ok {
		return nil, errors.New("error transforming to cpu cores")
	}

	return float64(v) / 1000000000, nil
}

func fromNanoToMilli(value definition.FetchedValue) (definition.FetchedValue, error) {
	v, ok := value.(uint64)
	if !ok {
		return nil, errors.New("error transforming cpu cores to milliseconds")
	}

	return float64(v) / 1000000, nil
}

func toTimestamp(value definition.FetchedValue) (definition.FetchedValue, error) {
	v, ok := value.(time.Time)
	if !ok {
		return nil, errors.New("error transforming to timestamp")
	}

	return v.Unix(), nil
}

func toNumericBoolean(value definition.FetchedValue) (definition.FetchedValue, error) {
	switch value {
	case "true", "True", true, 1:
		return 1, nil
	case "false", "False", false, 0:
		return 0, nil
	case "unknown":
		return -1, nil
	default:
		return nil, fmt.Errorf("value '%v' can not be converted to numeric boolean", value)
	}
}

func toCores(value definition.FetchedValue) (definition.FetchedValue, error) {
	switch v := value.(type) {
	case int:
		return float64(v) / 1000, nil
	case int64:
		return float64(v) / 1000, nil
	default:
		return nil, errors.New("error transforming to cores")
	}
}

func fromPrometheusNumeric(value definition.FetchedValue) (definition.FetchedValue, error) {
	switch v := value.(type) {
	case prometheus.GaugeValue:
		return float64(v), nil
	case prometheus.CounterValue:
		return float64(v), nil
	}

	return nil, fmt.Errorf("invalid type value '%v'. Expected 'gauge' or 'counter', got '%T'", value, value)
}

// Subtract returns a new FetchFunc that subtracts 2 values. It expects that the values are float64
func Subtract(left definition.FetchFunc, right definition.FetchFunc) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		leftValue, err := left(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}
		rightValue, err := right(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		result := leftValue.(float64) - rightValue.(float64)
		return result, nil
	}
}

// fetchWithDefault provides a default whenever a metric is missing
func fetchWithDefault(fetch definition.FetchFunc, defaultValue definition.FetchedValue) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		value, err := fetch(groupLabel, entityID, groups)
		if err != nil {
			// The error is currently discarded completely,
			// we could decide to log this, but it would likely be very noisy and not useful
			return defaultValue, nil
		}

		return value, nil
	}
}

// fetchIfMissing fetch replacement only if main metric is not present
// Example: `fetchIfMissing(definition.FromRaw("a"), definition.FromRaw("b"))` will only fetch metric "a" if "b"
// is missing. When replacement is not fetched, it returns an empty `FetchedValues`.
func fetchIfMissing(replacement definition.FetchFunc, main definition.FetchFunc) definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		_, errWhenMissing := main(groupLabel, entityID, groups)
		if errWhenMissing == nil {
			return definition.FetchedValues{}, nil
		}
		return replacement(groupLabel, entityID, groups)
	}
}

// metricSetTypeGuesserWithCustomGroup customizes K8sMetricSetTypeGuesser by setting up a custom value instead of the
// groupLabel.
func metricSetTypeGuesserWithCustomGroup(group string) definition.GuessFunc {
	return func(_ string) (string, error) {
		return definition.K8sMetricSetTypeGuesser(group) //nolint: wrapcheck
	}
}

// error checks.
var (
	errFetchedValueTypeCheck = fmt.Errorf("fetchedValue must be of type float64")
	errCPULimitTypeCheck     = fmt.Errorf("cpuLimit must be of type float64")
	errGroupLabelCheck       = fmt.Errorf("group label not found")
	errEntityCheck           = fmt.Errorf("entity Id not found")
	errHighCPUUsedCores      = fmt.Errorf("impossibly high value received from kubelet for cpuUsedCoresVal")
)

// filterCPUUsedCores checks for the correctness of the container metric cpuUsedCores returned by kubelet.
// cpuUsedCores a.k.a `usageNanoCores` value is set by cAdvisor and is returned by kubelet stats summary endpoint.
//
//nolint:nolintlint,ireturn
func filterCPUUsedCores(fetchedValue definition.FetchedValue, groupLabel, entityID string, groups definition.RawGroups) (definition.FilteredValue, error) {
	// type assertion check
	val, ok := fetchedValue.(float64)
	if !ok {
		return nil, errFetchedValueTypeCheck
	}

	// fetch raw cpuLimitCores value
	group, ok := groups[groupLabel]
	if !ok {
		return nil, errGroupLabelCheck
	}

	entity, ok := group[entityID]
	if !ok {
		return nil, errEntityCheck
	}

	value, ok := entity["cpuLimitCores"]
	if !ok {
		// there is likely no CPU limit set for the container which means we have to assume a reasonable value
		// since there is no way to know the max cpu cores for the current node, use default max of 96 cores supported by most cloud providers
		// a higher value wouldn't hurt our calculation as the cpuUsedCores value will be a super high number
		value = 96000 // 96 * 1000m k8s cpu unit
	}

	// apply transform before comparisons
	cpuLimitCoresVal, err := toCores(value)
	if err != nil {
		return nil, err
	}

	// check type assertion
	cpuLimit, ok := cpuLimitCoresVal.(float64)
	if !ok {
		return nil, errCPULimitTypeCheck
	}

	// check for impossibly high cpuUsedCoresVal - workaround for https://github.com/kubernetes/kubernetes/issues/114057 (resolved)
	if val > cpuLimit*100 {
		return nil, errHighCPUUsedCores
	}

	// return valid raw value
	return fetchedValue, nil
}
