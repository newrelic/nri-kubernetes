// Package cloud detects the managed Kubernetes cluster name from the underlying
// cloud provider (GKE, AKS, EKS). Detection is best-effort: any failure returns an
// empty name so callers proceed without the cloud.k8s.cluster.name attribute.
package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

// Provider identifies a cloud provider derived from a node's spec.providerID.
type Provider string

const (
	ProviderGKE     Provider = "GKE"
	ProviderEKS     Provider = "EKS"
	ProviderAKS     Provider = "AKS"
	ProviderUnknown Provider = ""
)

const defaultHTTPTimeout = 2 * time.Second

// DetectClusterId reads the node's spec.providerID to pick the provider, then
// queries that provider's metadata for the cluster id. Errors are returned for
// logging only; the caller treats an empty name as "not detected".
func DetectClusterId(ctx context.Context, logger *log.Logger, k8s kubernetes.Interface, nodeName string) (string, Provider, error) {
	if logger == nil {
		logger = logutil.Discard
	}
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", ProviderUnknown, fmt.Errorf("getting node %q: %w", nodeName, err)
	}
	provider, region, instanceID := parseProviderID(node.Spec.ProviderID)

	switch provider {
	case ProviderGKE:
		name, err := detectGKE(ctx)
		return name, provider, err
	case ProviderAKS:
		name, err := detectAKS(ctx, logger, node)
		return name, provider, err
	case ProviderEKS:
		name, err := detectEKS(ctx, logger, region, instanceID)
		return name, provider, err
	default:
		return "", ProviderUnknown, fmt.Errorf("unrecognized providerID %q", node.Spec.ProviderID)
	}
}

// parseProviderID inspects a Kubernetes node spec.providerID and returns the cloud
// provider plus, for AWS, the region and instance-id encoded in it.
//
//	gce://<project>/<zone>/<instance>
//	aws:///<az>/<instance-id>
//	azure:///subscriptions/.../virtualMachines/<vm>
func parseProviderID(providerID string) (Provider, string, string) {
	switch {
	case strings.HasPrefix(providerID, "gce://"):
		return ProviderGKE, "", ""
	case strings.HasPrefix(providerID, "aws://"):
		region, instanceID := parseAWSProviderID(providerID)
		return ProviderEKS, region, instanceID
	case strings.HasPrefix(providerID, "azure://"):
		return ProviderAKS, "", ""
	default:
		return ProviderUnknown, "", ""
	}
}

// parseAWSProviderID extracts region and instance-id from aws:///<az>/<instance-id>.
func parseAWSProviderID(providerID string) (region, instanceID string) {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(providerID, "aws://"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", ""
	}
	az := parts[len(parts)-2]
	instanceID = parts[len(parts)-1]
	return azToRegion(az), instanceID
}

// azToRegion strips the trailing AZ letter from an availability zone (us-east-1a -> us-east-1).
func azToRegion(az string) string {
	if az == "" {
		return ""
	}
	if last := az[len(az)-1]; last >= 'a' && last <= 'z' {
		return az[:len(az)-1]
	}
	return az
}
