package cloud

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var errUnsupportedAKSResourceID = errors.New("cannot assemble AKS resource id from providerID (custom node resource groups and names with underscores are unsupported)")

// nodeGroupComponentCount is the number of parts in a node resource group named.
const nodeGroupComponentCount = 4

// aksDetector resolves the AKS cluster ARM id purely from the node providerID.
type aksDetector struct {
	logger *log.Logger
}

// detect assembles /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.ContainerService/managedClusters/<cluster>.
// All inputs come from the node's spec.providerID; the resource group and cluster name
// are parsed from the node (managed) resource group MC_<rg>_<cluster>_<region>. This is
// best-effort: it breaks with a custom --node-resource-group, or when the resource group
// or cluster name contains underscores.
func (a aksDetector) detect(_ context.Context, providerID string) (string, error) {
	subscriptionID, nodeRG := parseAzureProviderID(providerID)
	resourceGroup, cluster := parseAKSNodeResourceGroup(nodeRG)

	if subscriptionID == "" || resourceGroup == "" || cluster == "" {
		return "", fmt.Errorf("%w: %q", errUnsupportedAKSResourceID, providerID)
	}

	id := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s",
		subscriptionID, resourceGroup, cluster)
	a.logger.Debugf("cloud: assembled AKS resource id %q from node resource group %q (best-effort)", id, nodeRG)
	return id, nil
}

// parseAzureProviderID extracts the subscription id and node resource group from providerID.
func parseAzureProviderID(providerID string) (subscriptionID, nodeResourceGroup string) {
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
