package scenario

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/version"

	"github.com/newrelic/nri-kubernetes/v2/e2e/jsonschema"
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

func (s Scenario) HelmValues() []string {
	base := []string{
		fmt.Sprintf("rbac=%v", s.rbac),
		fmt.Sprintf("ksm-instance-one.rbac.create=%v", s.rbac),
		fmt.Sprintf("ksm-instance-one.image.tag=%s", s.ksmVersion),
		fmt.Sprintf("daemonset.unprivileged=%v", s.unprivileged),
		fmt.Sprintf("daemonset.image.repository=%s", s.integrationImageRepository),
		fmt.Sprintf("daemonset.image.tag=%s", s.integrationImageTag),
		fmt.Sprintf("daemonset.clusterFlavor=%s", s.ClusterFlavor),
		fmt.Sprintf("k8sversion=%s", s.K8sVersion),
	}

	if s.twoKSMInstances {
		base = append(base, []string{
			fmt.Sprintf("ksm-instance-two.rbac.create=%v", s.rbac),
			fmt.Sprintf("ksm-instance-two.image.tag=%s", s.ksmVersion),
			"two-ksm-instances=true",
		}...)
	}

	return base
}

func (s Scenario) String() string {
	return strings.Join(s.HelmValues(), ",")
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
