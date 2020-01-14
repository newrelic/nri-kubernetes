package metric

import (
	"errors"
	"fmt"
	"time"

	sdkMetric "github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/newrelic/nri-kubernetes/src/definition"
	ksmMetric "github.com/newrelic/nri-kubernetes/src/ksm/metric"
	kubeletMetric "github.com/newrelic/nri-kubernetes/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
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
			{
				Name:      "etcdObjectCounts",
				ValueFunc: prometheus.FromValueWithOverriddenName("etcd_object_counts", "etcdObjectCounts"),
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
				ValueFunc: prometheus.FromValueWithOverriddenName("process_open_fds", "processOpenFds"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "processMaxFds",
				ValueFunc: prometheus.FromValueWithOverriddenName("process_max_fds", "processMaxFds"),
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
	"replicaset": {
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_replicaset_created", "replicaset"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_replicaset_created"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_replicaset_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_replicaset_spec_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_replicaset_status_ready_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsTotal", ValueFunc: prometheus.FromValue("kube_replicaset_status_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsFullyLabeled", ValueFunc: prometheus.FromValue("kube_replicaset_status_fully_labeled_replicas"), Type: sdkMetric.GAUGE},
			{Name: "observedGeneration", ValueFunc: prometheus.FromValue("kube_replicaset_status_observed_generation"), Type: sdkMetric.GAUGE},
			{Name: "replicasetName", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "replicaset"), Type: sdkMetric.ATTRIBUTE},
			// namespace is here for backwards compatibility, we should use the namespaceName
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_replicaset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "deploymentName", ValueFunc: ksmMetric.GetDeploymentNameForReplicaSet(), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"statefulset": {
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_statefulset_created", "statefulset"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_statefulset_created"),
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
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("statefulset", "kube_statefulset_labels"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"daemonset": {
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_daemonset_created", "daemonset"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_daemonset_created"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_daemonset_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_daemonset_status_desired_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsScheduled", ValueFunc: prometheus.FromValue("kube_daemonset_status_current_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsAvailable", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_available"), Type: sdkMetric.GAUGE},
			{Name: "podsReady", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_ready"), Type: sdkMetric.GAUGE},
			{Name: "podsUnavailable", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "podsMisscheduled", ValueFunc: prometheus.FromValue("kube_daemonset_status_number_misscheduled"), Type: sdkMetric.GAUGE},
			{Name: "podsUpdatedScheduled", ValueFunc: prometheus.FromValue("kube_daemonset_updated_number_scheduled"), Type: sdkMetric.GAUGE},
			{Name: "metadataGeneration", ValueFunc: prometheus.FromValue("kube_daemonset_metadata_generation"), Type: sdkMetric.GAUGE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_daemonset_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("daemonset", "kube_daemonset_labels"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"namespace": {
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_namespace_created"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_namespace_created"), Type: sdkMetric.GAUGE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_namespace_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_namespace_created", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "status", ValueFunc: prometheus.FromLabelValue("kube_namespace_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("namespace", "kube_namespace_labels"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"deployment": {
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_deployment_created", "deployment"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_deployment_created"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_deployment_created"), Type: sdkMetric.GAUGE},
			{Name: "podsDesired", ValueFunc: prometheus.FromValue("kube_deployment_spec_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsTotal", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas"), Type: sdkMetric.GAUGE},
			{Name: "podsAvailable", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_available"), Type: sdkMetric.GAUGE},
			{Name: "podsUnavailable", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "podsUpdated", ValueFunc: prometheus.FromValue("kube_deployment_status_replicas_updated"), Type: sdkMetric.GAUGE},
			{Name: "podsMaxUnavailable", ValueFunc: prometheus.FromValue("kube_deployment_spec_strategy_rollingupdate_max_unavailable"), Type: sdkMetric.GAUGE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_deployment_labels", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_deployment_labels", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "deploymentName", ValueFunc: prometheus.FromLabelValue("kube_deployment_labels", "deployment"), Type: sdkMetric.ATTRIBUTE},
			// Important: The order of these lines is important: we could have the same label in different entities, and we would like to keep the value closer to deployment
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("namespace", "kube_namespace_labels"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("deployment", "kube_deployment_labels"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"service": {
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_service_created", "service"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_service_created"),
		Specs: []definition.Spec{
			{
				Name:      "createdAt",
				ValueFunc: prometheus.FromValue("kube_service_created"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "namespaceName",
				ValueFunc: prometheus.FromLabelValue("kube_service_labels", "namespace"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "serviceName",
				ValueFunc: prometheus.FromLabelValue("kube_service_labels", "service"),
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
				ValueFunc: prometheus.InheritAllLabelsFrom("service", "kube_service_labels"),
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
		IDGenerator:   prometheus.FromLabelValueEntityIDGenerator("kube_endpoint_created", "endpoint"),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_endpoint_created"),
		Specs: []definition.Spec{
			{
				Name:      "createdAt",
				ValueFunc: prometheus.FromValue("kube_endpoint_created"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "namespaceName",
				ValueFunc: prometheus.FromLabelValue("kube_endpoint_labels", "namespace"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "endpointName",
				ValueFunc: prometheus.FromLabelValue("kube_endpoint_labels", "endpoint"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "label.*",
				ValueFunc: prometheus.InheritAllLabelsFrom("endpoint", "kube_endpoint_labels"),
				Type:      sdkMetric.ATTRIBUTE,
			},
			{
				Name:      "addressNotReady",
				ValueFunc: prometheus.FromValue("kube_endpoint_address_not_ready"),
				Type:      sdkMetric.GAUGE,
			},
			{
				Name:      "addressAvailable",
				ValueFunc: prometheus.FromValue("kube_endpoint_address_available"),
				Type:      sdkMetric.GAUGE,
			},
		},
	},
	// We get Pod metrics from kube-state-metrics for those pods that are in
	// "Pending" status and are not scheduled. We can't get the data from Kubelet because
	// they aren't running in any node and the information about them is only
	// present in the API.
	"pod": {
		IDGenerator:   prometheus.FromLabelsValueEntityIDGeneratorForPendingPods(),
		TypeGenerator: prometheus.FromLabelValueEntityTypeGenerator("kube_pod_status_phase"),
		Specs: []definition.Spec{
			{Name: "createdAt", ValueFunc: prometheus.FromValue("kube_pod_created"), Type: sdkMetric.GAUGE},
			{Name: "startTime", ValueFunc: prometheus.FromValue("kube_pod_start_time"), Type: sdkMetric.GAUGE},
			{Name: "createdKind", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "created_by_kind"), Type: sdkMetric.ATTRIBUTE},
			{Name: "createdBy", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "created_by_name"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeIP", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "host_ip"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "node"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podName", ValueFunc: prometheus.FromLabelValue("kube_pod_info", "pod"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isReady", ValueFunc: definition.Transform(prometheus.FromLabelValue("kube_pod_status_ready", "condition"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "status", ValueFunc: prometheus.FromLabelValue("kube_pod_status_phase", "phase"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isScheduled", ValueFunc: definition.Transform(prometheus.FromLabelValue("kube_pod_status_scheduled", "condition"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "deploymentName", ValueFunc: ksmMetric.GetDeploymentNameForPod(), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: prometheus.InheritAllLabelsFrom("pod", "kube_pod_labels"), Type: sdkMetric.ATTRIBUTE},
		},
	},
}

// KSMQueries are the queries we will do to KSM in order to fetch all the raw metrics.
var KSMQueries = []prometheus.Query{
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
	{MetricName: "kube_daemonset_updated_number_scheduled"},
	{MetricName: "kube_daemonset_metadata_generation"},
	{MetricName: "kube_daemonset_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_replicaset_spec_replicas"},
	{MetricName: "kube_replicaset_status_ready_replicas"},
	{MetricName: "kube_replicaset_status_replicas"},
	{MetricName: "kube_replicaset_status_fully_labeled_replicas"},
	{MetricName: "kube_replicaset_status_observed_generation"},
	{MetricName: "kube_replicaset_created"},
	{MetricName: "kube_namespace_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_namespace_created"},
	{MetricName: "kube_namespace_status_phase", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_deployment_labels", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_deployment_created"},
	{MetricName: "kube_deployment_spec_replicas"},
	{MetricName: "kube_deployment_status_replicas"},
	{MetricName: "kube_deployment_status_replicas_available"},
	{MetricName: "kube_deployment_status_replicas_unavailable"},
	{MetricName: "kube_deployment_status_replicas_updated"},
	{MetricName: "kube_deployment_spec_strategy_rollingupdate_max_unavailable"},
	{MetricName: "kube_pod_status_phase", Labels: prometheus.QueryLabels{
		Labels: prometheus.Labels{"phase": "Pending"},
	}, Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_info"},
	{MetricName: "kube_pod_created"},
	{MetricName: "kube_pod_labels"},
	{MetricName: "kube_pod_status_scheduled", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_status_ready", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_pod_start_time"},
	{MetricName: "kube_service_created"},
	{MetricName: "kube_service_labels"},
	{MetricName: "kube_service_info"},
	{MetricName: "kube_service_spec_type", Value: prometheus.QueryValue{
		Value: prometheus.GaugeValue(1),
	}},
	{MetricName: "kube_endpoint_created"},
	{MetricName: "kube_endpoint_labels"},
	{MetricName: "kube_endpoint_address_not_ready"},
	{MetricName: "kube_endpoint_address_available"},
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
}

// KubeletSpecs are the metric specifications we want to collect from Kubelet.
var KubeletSpecs = definition.SpecGroups{
	"pod": {
		IDGenerator:   kubeletMetric.FromRawEntityIDGroupEntityIDGenerator("namespace"),
		TypeGenerator: kubeletMetric.FromRawGroupsEntityTypeGenerator,
		Specs: []definition.Spec{
			// /stats/summary endpoint
			{Name: "net.rxBytesPerSecond", ValueFunc: definition.FromRaw("rxBytes"), Type: sdkMetric.RATE},
			{Name: "net.txBytesPerSecond", ValueFunc: definition.FromRaw("txBytes"), Type: sdkMetric.RATE},
			{Name: "net.errorsPerSecond", ValueFunc: definition.FromRaw("errors"), Type: sdkMetric.RATE},

			// /pods endpoint
			{Name: "createdAt", ValueFunc: definition.Transform(definition.FromRaw("createdAt"), toTimestamp), Type: sdkMetric.GAUGE},
			{Name: "startTime", ValueFunc: definition.Transform(definition.FromRaw("startTime"), toTimestamp), Type: sdkMetric.GAUGE},
			{Name: "createdKind", ValueFunc: definition.FromRaw("createdKind"), Type: sdkMetric.ATTRIBUTE},
			{Name: "createdBy", ValueFunc: definition.FromRaw("createdBy"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeIP", ValueFunc: definition.FromRaw("nodeIP"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isReady", ValueFunc: definition.Transform(definition.FromRaw("isReady"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "status", ValueFunc: definition.FromRaw("status"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isScheduled", ValueFunc: definition.Transform(definition.FromRaw("isScheduled"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "deploymentName", ValueFunc: definition.FromRaw("deploymentName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "label.*", ValueFunc: definition.Transform(definition.FromRaw("labels"), kubeletMetric.OneMetricPerLabel), Type: sdkMetric.ATTRIBUTE},
			{Name: "reason", ValueFunc: definition.FromRaw("reason"), Type: sdkMetric.ATTRIBUTE},
			{Name: "message", ValueFunc: definition.FromRaw("message"), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"container": {
		IDGenerator:   kubeletMetric.FromRawGroupsEntityIDGenerator("containerName"),
		TypeGenerator: kubeletMetric.FromRawGroupsEntityTypeGenerator,
		Specs: []definition.Spec{
			// /stats/summary endpoint
			{Name: "memoryUsedBytes", ValueFunc: definition.FromRaw("usageBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryWorkingSetBytes", ValueFunc: definition.FromRaw("workingSetBytes"), Type: sdkMetric.GAUGE},
			{Name: "cpuUsedCores", ValueFunc: definition.Transform(definition.FromRaw("usageNanoCores"), fromNano), Type: sdkMetric.GAUGE},
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

			// /pods endpoint
			{Name: "containerName", ValueFunc: definition.FromRaw("containerName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "containerImage", ValueFunc: definition.FromRaw("containerImage"), Type: sdkMetric.ATTRIBUTE},
			{Name: "deploymentName", ValueFunc: definition.FromRaw("deploymentName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "nodeIP", ValueFunc: definition.FromRaw("nodeIP"), Type: sdkMetric.ATTRIBUTE},
			{Name: "restartCount", ValueFunc: definition.FromRaw("restartCount"), Type: sdkMetric.GAUGE},
			{Name: "cpuRequestedCores", ValueFunc: definition.Transform(definition.FromRaw("cpuRequestedCores"), toCores), Type: sdkMetric.GAUGE},
			{Name: "cpuLimitCores", ValueFunc: definition.Transform(definition.FromRaw("cpuLimitCores"), toCores), Type: sdkMetric.GAUGE},
			{Name: "memoryRequestedBytes", ValueFunc: definition.FromRaw("memoryRequestedBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryLimitBytes", ValueFunc: definition.FromRaw("memoryLimitBytes"), Type: sdkMetric.GAUGE},
			{Name: "status", ValueFunc: definition.FromRaw("status"), Type: sdkMetric.ATTRIBUTE},
			{Name: "isReady", ValueFunc: definition.Transform(definition.FromRaw("isReady"), toNumericBoolean), Type: sdkMetric.GAUGE},
			{Name: "reason", ValueFunc: definition.FromRaw("reason"), Type: sdkMetric.ATTRIBUTE}, // Previously called statusWaitingReason

			// Inherit from pod
			{Name: "label.*", ValueFunc: definition.Transform(definition.FromRaw("labels"), kubeletMetric.OneMetricPerLabel), Type: sdkMetric.ATTRIBUTE},
		},
	},
	"node": {
		TypeGenerator: kubeletMetric.FromRawGroupsEntityTypeGenerator,
		Specs: []definition.Spec{
			{Name: "nodeName", ValueFunc: definition.FromRaw("nodeName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "cpuUsedCores", ValueFunc: definition.Transform(definition.FromRaw("usageNanoCores"), fromNano), Type: sdkMetric.GAUGE},
			{Name: "cpuUsedCoreMilliseconds", ValueFunc: definition.Transform(definition.FromRaw("usageCoreNanoSeconds"), fromNanoToMilli), Type: sdkMetric.GAUGE},
			{Name: "memoryUsedBytes", ValueFunc: definition.FromRaw("memoryUsageBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryAvailableBytes", ValueFunc: definition.FromRaw("memoryAvailableBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryWorkingSetBytes", ValueFunc: definition.FromRaw("memoryWorkingSetBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryRssBytes", ValueFunc: definition.FromRaw("memoryRssBytes"), Type: sdkMetric.GAUGE},
			{Name: "memoryPageFaults", ValueFunc: definition.FromRaw("memoryPageFaults"), Type: sdkMetric.GAUGE},
			{Name: "memoryMajorPageFaultsPerSecond", ValueFunc: definition.FromRaw("memoryMajorPageFaults"), Type: sdkMetric.RATE},
			{Name: "net.rxBytesPerSecond", ValueFunc: definition.FromRaw("rxBytes"), Type: sdkMetric.RATE},
			{Name: "net.txBytesPerSecond", ValueFunc: definition.FromRaw("txBytes"), Type: sdkMetric.RATE},
			{Name: "net.errorsPerSecond", ValueFunc: definition.FromRaw("errors"), Type: sdkMetric.RATE},
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
		},
	},
	"volume": {
		TypeGenerator: kubeletMetric.FromRawGroupsEntityTypeGenerator,
		Specs: []definition.Spec{
			{Name: "volumeName", ValueFunc: definition.FromRaw("volumeName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "podName", ValueFunc: definition.FromRaw("podName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespace", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "namespaceName", ValueFunc: definition.FromRaw("namespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "persistent", ValueFunc: isPersistentVolume(), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcName", ValueFunc: definition.FromRaw("pvcName"), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcNamespace", ValueFunc: definition.FromRaw("pvcNamespace"), Type: sdkMetric.ATTRIBUTE},
			{Name: "pvcNamespaceName", ValueFunc: definition.FromRaw("pvcNamespace"), Type: sdkMetric.ATTRIBUTE},
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

func isPersistentVolume() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		name, err := definition.FromRaw("pvcName")(groupLabel, entityID, groups)
		if err == nil && name != "" {
			return "true", nil
		}
		return "false", nil
	}
}

func computePercentage(current, all uint64) (definition.FetchedValue, error) {
	if all == uint64(0) {
		return nil, errors.New("division by zero")
	}
	return ((float64(current) / float64(all)) * 100), nil
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
	default:
		return nil, errors.New("value can not be converted to numeric boolean")
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
