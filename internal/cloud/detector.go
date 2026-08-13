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

type Provider string

const (
	ProviderGKE     Provider = "GKE"
	ProviderEKS     Provider = "EKS"
	ProviderAKS     Provider = "AKS"
	ProviderUnknown Provider = ""
)

const defaultHTTPTimeout = 2 * time.Second

// DetectClusterId reads the node's spec.providerID to pick the provider, then
// assembles that provider's cluster resource id (EKS ARN, AKS ARM id, GKE link).
// Errors are returned for logging only; the caller treats an empty id as "not detected".
func DetectClusterId(ctx context.Context, logger *log.Logger, k8s kubernetes.Interface, nodeName string) (string, Provider, error) {
	if logger == nil {
		logger = logutil.Discard
	}
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", ProviderUnknown, fmt.Errorf("getting node %q: %w", nodeName, err)
	}
	providerID := node.Spec.ProviderID

	switch {
	case strings.HasPrefix(providerID, "gce://"):
		id, err := detectGKE(ctx, providerID)
		return id, ProviderGKE, err
	case strings.HasPrefix(providerID, "aws://"):
		id, err := detectEKS(ctx, logger, providerID)
		return id, ProviderEKS, err
	case strings.HasPrefix(providerID, "azure://"):
		id, err := detectAKS(logger, providerID)
		return id, ProviderAKS, err
	default:
		return "", ProviderUnknown, fmt.Errorf("unrecognized providerID %q", providerID)
	}
}
