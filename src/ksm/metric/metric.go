package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const DEPLOYMENT_OWNER_KIND string = "Deployment"

// GetDeploymentNameForReplicaSet returns the name of the deployment that owns
// a ReplicaSet, or returns an error if the owner is not a deployment.
func GetDeploymentNameForReplicaSet() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		ownerKindRawVal, err := prometheus.FromLabelValue("kube_replicaset_owner", "owner_kind")(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		ownerKind, ok := ownerKindRawVal.(string)
		if !ok {
			return nil, errors.New("error retrieving deployment name for replica set. failed to convert owner_kind field to string")
		}

		if ownerKind != DEPLOYMENT_OWNER_KIND {
			return nil, fmt.Errorf("error retrieving deployment name for replica set. its owner_kind ('%s') is not '%s'", ownerKind, DEPLOYMENT_OWNER_KIND)
		}

		ownerNameRawVal, err := prometheus.FromLabelValue("kube_replicaset_owner", "owner_name")(groupLabel, entityID, groups)
		if err != nil {
			return nil, err
		}

		ownerName, ok := ownerNameRawVal.(string)
		if !ok {
			return nil, errors.New("error retrieving deployment name for replica set. failed to convert owner_name field to string")
		}

		if ownerName == "" {
			return nil, errors.New("error retrieving deployment name for replica set. owner_name field is empty")
		}

		return ownerName, nil
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
