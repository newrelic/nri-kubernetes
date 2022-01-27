package metric

import (
	"errors"
	"strings"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

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
