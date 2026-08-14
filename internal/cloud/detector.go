package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

var errUnrecognizedProviderID = errors.New("unrecognized providerID")

type Provider string

const (
	ProviderGKE     Provider = "GKE"
	ProviderEKS     Provider = "EKS"
	ProviderAKS     Provider = "AKS"
	ProviderUnknown Provider = ""
)

const defaultHTTPTimeout = 2 * time.Second

// providerDetector enforces required methods for each cloud detector.
type providerDetector interface {
	detect(ctx context.Context, providerID string) (resourceID string, detectionError error)
}

// Detector routes a node's spec.providerID to the matching cloud provider detector.
type Detector struct {
	gke providerDetector
	eks providerDetector
	aks providerDetector
}

func NewDetector(logger *log.Logger) *Detector {
	if logger == nil {
		logger = logutil.Discard
	}
	return &Detector{
		gke: gkeDetector{metadataBaseURL: defaultGKEMetadataBaseURL},
		eks: eksDetector{newEC2Client: defaultNewEC2Client},
		aks: aksDetector{logger: logger},
	}
}

// DetectClusterID reads the node's spec.providerID to pick the provider, then
// assembles that provider's cluster resource id (EKS ARN, AKS ARM id, GKE link).
// Errors are returned for logging only; the caller treats an empty id as "not detected".
func (d *Detector) DetectClusterID(ctx context.Context, k8s kubernetes.Interface, nodeName string) (string, Provider, error) {
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", ProviderUnknown, fmt.Errorf("getting node %q: %w", nodeName, err)
	}
	providerID := node.Spec.ProviderID

	switch {
	case strings.HasPrefix(providerID, "gce://"):
		id, err := d.gke.detect(ctx, providerID)
		return id, ProviderGKE, err
	case strings.HasPrefix(providerID, "aws://"):
		id, err := d.eks.detect(ctx, providerID)
		return id, ProviderEKS, err
	case strings.HasPrefix(providerID, "azure://"):
		id, err := d.aks.detect(ctx, providerID)
		return id, ProviderAKS, err
	default:
		return "", ProviderUnknown, fmt.Errorf("%w: %q", errUnrecognizedProviderID, providerID)
	}
}
