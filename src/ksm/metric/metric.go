package metric

import (
	"errors"
	"fmt"

	"strings"

	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

// PrometheusMetricsPath is the KSM prometheus metrics endpoint.
const PrometheusMetricsPath = "/metrics"

// GetStatusForContainer returns the status of a container
func GetStatusForContainer() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		queryValue := prometheus.GaugeValue(1)
		s := []string{"running", "waiting", "terminated"}
		for _, k := range s {
			v, _ := prometheus.FromValue(fmt.Sprintf("kube_pod_container_status_%s", k))(groupLabel, entityID, groups)
			if v == queryValue {
				return strings.Title(k), nil
			}
		}

		return "Unknown", nil
	}
}

// GetDeploymentNameForReplicaSet returns the name of the deployment has created
// a ReplicaSet.
func GetDeploymentNameForReplicaSet() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		replicasetName, err := prometheus.FromLabelValue("kube_replicaset_created", "replicaset")(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		if replicasetName.(string) == "" {
			return nil, errors.New("error generating deployment name for replica set. replicaset field is empty")
		}

		return replicasetNameToDeploymentName(replicasetName.(string)), nil
	}
}

// GetDeploymentNameForPod returns the name of the deployment has created a
// Pod.  It returns an empty string if Pod hasn't been created by a deployment.
func GetDeploymentNameForPod() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		creatorKind, err := prometheus.FromLabelValue("kube_pod_info", "created_by_kind")(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		if creatorKind.(string) == "" {
			return nil, errors.New("error generating deployment name for pod. created_by_kind field is empty")
		}

		creatorName, err := prometheus.FromLabelValue("kube_pod_info", "created_by_name")(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		if creatorName.(string) == "" {
			return nil, errors.New("error generating deployment name for pod. created_by_name field is empty")
		}

		return deploymentNameBasedOnCreator(creatorKind.(string), creatorName.(string)), nil
	}
}

// GetDeploymentNameForContainer returns the name of the deployment has created
// a container. It's providing this information inheriting some metrics from its
// pod. Returns an empty string if its pod hasn't been created by a deployment.
func GetDeploymentNameForContainer() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		mm := map[string]string{
			"created_by_kind": "created_by_kind",
			"created_by_name": "created_by_name",
		}
		podValues, err := prometheus.InheritSpecificLabelValuesFrom("pod", "kube_pod_info", mm)(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}
		podMetrics := podValues.(definition.FetchedValues)
		if _, ok := podMetrics["created_by_kind"].(string); !ok || podMetrics["created_by_kind"].(string) == "" {
			return nil, errors.New("error generating deployment name for container. created_by_kind field is missing")
		}

		if _, ok := podMetrics["created_by_name"].(string); !ok || podMetrics["created_by_name"].(string) == "" {
			return nil, errors.New("error generating deployment name for container. created_by_name field is missing")
		}

		return deploymentNameBasedOnCreator(podMetrics["created_by_kind"].(string), podMetrics["created_by_name"].(string)), nil

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

// UnscheduledItemsPatcher adds to the destination RawGroups the pods that haven't been scheduled
func UnscheduledItemsPatcher(destination definition.RawGroups, source definition.RawGroups) {
	for podName, pod := range source["pod"] {
		if _, ok := destination["pod"][podName]; !ok {
			podMap := pod["kube_pod_info"].(prometheus.Metric).Labels
			if podMap["node"] == "" {
				destination["pod"][podName] = definition.RawMetrics{}
				destination["pod"][podName]["podName"] = podMap["pod"]
				destination["pod"][podName]["namespace"] = podMap["namespace"]
			}
		}
	}
}
