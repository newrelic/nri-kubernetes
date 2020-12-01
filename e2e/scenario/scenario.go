package scenario

import (
	"fmt"
	"strconv"

	"github.com/newrelic/nri-kubernetes/e2e/jsonschema"
	"k8s.io/apimachinery/pkg/version"
)

// Scenario defines the environment that will be used for testing
type Scenario struct {
	unprivileged               bool
	rbac                       bool
	ksmVersion                 string
	twoKSMInstances            bool
	integrationImageRepository string
	integrationImageTag        string
	optionalNetworkSchema      bool
	ClusterFlavor              string
	K8sVersion                 string
}

// New returns a new Scenario
func New(
	rbac bool,
	unprivileged bool,
	integrationImageRepository,
	integrationImageTag,
	ksmVersion string,
	twoKSMInstances bool,
	k8sServerInfo *version.Info,
	clusterFlavor string,
	K8sVersion string,
) Scenario {
	return Scenario{
		unprivileged:               unprivileged,
		rbac:                       rbac,
		ksmVersion:                 ksmVersion,
		twoKSMInstances:            twoKSMInstances,
		integrationImageRepository: integrationImageRepository,
		integrationImageTag:        integrationImageTag,
		optionalNetworkSchema:      optionalNetworkSchema(k8sServerInfo, unprivileged),
		ClusterFlavor:              clusterFlavor,
		K8sVersion:                 K8sVersion,
	}
}

func (s Scenario) String() string {
	str := fmt.Sprintf(
		"rbac=%v,ksm-instance-one.rbac.create=%v,ksm-instance-one.image.tag=%s,daemonset.unprivileged=%v,daemonset.image.repository=%s,daemonset.image.tag=%s,daemonset.clusterFlavor=%s, k8sversion=%s",
		s.rbac,
		s.rbac,
		s.ksmVersion,
		s.unprivileged,
		s.integrationImageRepository,
		s.integrationImageTag,
		s.ClusterFlavor,
		s.K8sVersion,
	)
	if s.twoKSMInstances {
		return fmt.Sprintf(
			"%s,ksm-instance-two.rbac.create=%v,ksm-instance-two.image.tag=%s,two-ksm-instances=true",
			str,
			s.rbac,
			s.ksmVersion,
		)
	}
	return str
}

// GetSchemasForJob returns the json schemas that should be use to
// match the test scenario.
func (s Scenario) GetSchemasForJob(job string) jsonschema.EventTypeToSchemaFilename {

	eventTypeSchemas := defaultEventTypeToSchemaFilename()

	if s.optionalNetworkSchema {
		eventTypeSchemas["kubelet"]["K8sNodeSample"] = "node-no-network.json"
		eventTypeSchemas["kubelet"]["K8sPodSample"] = "pod-no-network.json"
	}

	return eventTypeSchemas[string(job)]
}

func defaultEventTypeToSchemaFilename() map[string]jsonschema.EventTypeToSchemaFilename {
	return map[string]jsonschema.EventTypeToSchemaFilename{
		"kube-state-metrics": {
			"K8sReplicasetSample":  "replicaset.json",
			"K8sNamespaceSample":   "namespace.json",
			"K8sDeploymentSample":  "deployment.json",
			"K8sDaemonsetSample":   "daemonset.json",
			"K8sStatefulsetSample": "statefulset.json",
			"K8sEndpointSample":    "endpoint.json",
			"K8sServiceSample":     "service.json",
		},
		"kubelet": {
			"K8sPodSample":       "pod.json",
			"K8sContainerSample": "container.json",
			"K8sNodeSample":      "node.json",
			"K8sVolumeSample":    "volume.json",
			"K8sClusterSample":   "cluster.json",
		},
		"scheduler": {
			"K8sSchedulerSample": "scheduler.json",
		},
		"etcd": {
			"K8sEtcdSample": "etcd.json",
		},
		"controller-manager": {
			"K8sControllerManagerSample": "controller-manager.json",
		},
		"api-server": {
			"K8sApiServerSample": "apiserver.json",
		},
	}
}

// optionalNetworkSchema returns true when kubernetes version is 1.18
// or newer and unprivileged is true.
func optionalNetworkSchema(k8sServerInfo *version.Info, unprivileged bool) bool {
	if k8sServerInfo == nil {
		return false
	}
	major, err := strconv.Atoi(k8sServerInfo.Major)
	if err != nil {
		return false
	}
	if major > 1 {
		return unprivileged
	}
	minor, err := strconv.Atoi(k8sServerInfo.Minor)
	if err != nil {
		return false
	}
	return minor >= 18 && unprivileged
}
