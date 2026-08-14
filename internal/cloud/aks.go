package cloud

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// nodeGroupComponentCount is the number of parts in a node resource group named.
const nodeGroupComponentCount = 4

// detectAKS assembles the AKS cluster ARM id
// /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.ContainerService/managedClusters/<cluster>.
//
// All inputs come from the node's spec.providerID:
//
// azure:///subscriptions/<sub>/resourceGroups/MC_<rg>_<cluster>_<region>/providers/Microsoft.Compute/...
//
// The cluster name and original resource group are parsed from the node (managed)
// resource group, which by default is named MC_<rg>_<cluster>_<region>. This is
// best-effort: it breaks with a custom --node-resource-group, or when the resource
// group or cluster name contains underscores.
var detectAKS = func(logger *log.Logger, providerID string) (string, error) {
	subscriptionID, nodeRG := parseAzureProviderID(providerID)
	resourceGroup, cluster := parseAKSNodeResourceGroup(nodeRG)

	if subscriptionID == "" || resourceGroup == "" || cluster == "" {
		return "", fmt.Errorf("cannot assemble AKS resource id from providerID %q (custom node resource groups and names with underscores are unsupported)", providerID)
	}

	id := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s",
		subscriptionID, resourceGroup, cluster)
	logger.Debugf("cloud: assembled AKS resource id %q from node resource group %q (best-effort)", id, nodeRG)
	return id, nil
}

// parseAzureProviderID extracts the subscription id and resource group from providerID.
func parseAzureProviderID(providerID string) (subscriptionID, resourceGroup string) {
	re := regexp.MustCompile(`^azure:///subscriptions/([a-zA-Z0-9._\-()]+)/resourceGroups/([a-zA-Z0-9._\-()]+)/providers/([a-zA-Z0-9._\-()]+)/virtualMachineScaleSets/([a-zA-Z0-9._\-()]+)/virtualMachines/([a-zA-Z0-9._\-()]+)$`)
	matches := re.FindStringSubmatch(providerID)
	if matches == nil {
		return "", ""
	}
	return matches[1], matches[2]
}

// parseAKSNodeResourceGroup splits a node resource group into its resource group and cluster name.
func parseAKSNodeResourceGroup(nodeRG string) (resourceGroup, cluster string) {
	if !strings.HasPrefix(strings.ToLower(nodeRG), "mc_") {
		return "", ""
	}
	parts := strings.Split(nodeRG, "_")
	if len(parts) != nodeGroupComponentCount {
		return "", ""
	}
	// MC_<resourceGroup>_<clusterName>_<region>
	return parts[1], parts[2]
}
