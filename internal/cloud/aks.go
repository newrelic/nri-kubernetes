package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// aksClusterLabel is a AKS node label whose value is the node resource group (MC_ form).
const aksClusterLabel = "kubernetes.azure.com/cluster"

// nodeGroupComponentCound is the number of parts in a node resource group MC_<resourceGroup>_<clusterName>_<region>.
const nodeGroupComponentCound = 4

// detectAKS attempts to parse the AKS cluster name from the node (managed) resource group,
// which by default is named MC_<resourceGroup>_<clusterName>_<region>. This is
// best-effort: it breaks when the cluster name or resource group contains underscores.
func detectAKS(ctx context.Context, logger *log.Logger, node *corev1.Node) (string, error) {
	rg, err := azureNodeResourceGroup(ctx)
	if err != nil {
		logger.Debugf("cloud: AKS IMDS lookup failed, falling back to node label: %v", err)
		rg = node.Labels[aksClusterLabel]
	}
	name := parseAKSClusterName(rg)
	if name == "" {
		return "", fmt.Errorf("cannot parse AKS cluster name from resource group %q (cluster names and resource group names with underscores are unsupported)", rg)
	}
	logger.Debugf("cloud: parsed AKS cluster name %q from %q (best-effort)", name, rg)
	return name, nil
}

func azureNodeResourceGroup(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, azureIMDSBaseURL+"/instance?api-version=2021-02-01", nil)
	if err != nil {
		return "", fmt.Errorf("building Azure IMDS request: %w", err)
	}
	req.Header.Set("Metadata", "true")

	body, err := doMetadataGet(req)
	if err != nil {
		return "", err
	}
	var meta struct {
		Compute struct {
			ResourceGroupName string `json:"resourceGroupName"`
		} `json:"compute"`
	}
	if err := json.Unmarshal([]byte(body), &meta); err != nil {
		return "", fmt.Errorf("decoding Azure IMDS response: %w", err)
	}
	return meta.Compute.ResourceGroupName, nil
}

// parseAKSClusterName extracts the cluster name from a node resource group named
// MC_<resourceGroup>_<clusterName>_<region>. Returns "" if it does not match.
func parseAKSClusterName(rg string) string {
	if !strings.HasPrefix(rg, "MC_") {
		return ""
	}
	parts := strings.Split(rg, "_")
	if len(parts) != nodeGroupComponentCound {
		return ""
	}
	// region is the last segment, cluster name is the second-to-last.
	return parts[len(parts)-2]
}
