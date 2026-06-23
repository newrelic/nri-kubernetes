package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

const deploymentOwnerKind = "Deployment"

var (
	ErrOwnerKindInvalid     = errors.New("failed to convert owner_kind of ReplicaSet to string")
	ErrNotOwnedByDeployment = errors.New("owner_kind of ReplicaSet is not " + deploymentOwnerKind)
	ErrOwnerNameInvalid     = errors.New("failed to convert owner_name of ReplicaSet to string")
	ErrOwnerNameEmpty       = errors.New("owner_name of ReplicaSet is empty")
)

// GetDeploymentNameForReplicaSet returns the name of the deployment that owns
// a ReplicaSet, an error if the owner is not a deployment.
func GetDeploymentNameForReplicaSet() definition.FetchFunc {
	return func(groupLabel, entityID string, groups definition.RawGroups) (definition.FetchedValue, error) {
		ownerKindRawVal, err := prometheus.FromLabelValue("kube_replicaset_owner", "owner_kind")(groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch owner_kind of ReplicaSet: %w", err)
		}

		ownerKind, ok := ownerKindRawVal.(string)
		if !ok {
			return nil, ErrOwnerKindInvalid
		}

		if ownerKind != deploymentOwnerKind {
			return nil, ErrNotOwnedByDeployment
		}

		ownerNameRawVal, err := prometheus.FromLabelValue("kube_replicaset_owner", "owner_name")(groupLabel, entityID, groups)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch owner_name of ReplicaSet: %w", err)
		}

		ownerName, ok := ownerNameRawVal.(string)
		if !ok {
			return nil, ErrOwnerNameInvalid
		}

		if ownerName == "" {
			return nil, ErrOwnerNameEmpty
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
