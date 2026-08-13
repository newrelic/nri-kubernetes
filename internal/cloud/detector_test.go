package cloud

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

// Test_DetectClusterId verifies that DetectClusterId routes to the correct provider
// based on the node's spec.providerID. The per-provider assembly logic is covered by
// Test_detectGKE / Test_detectEKS / Test_detectAKS; here the detectors are stubbed so
// the test asserts routing only.
func Test_DetectClusterId(t *testing.T) {
	const nodeName = "node-1"

	// Stub the provider detectors and restore them afterwards.
	origGKE, origEKS, origAKS := detectGKE, detectEKS, detectAKS
	defer func() { detectGKE, detectEKS, detectAKS = origGKE, origEKS, origAKS }()
	detectGKE = func(context.Context, string) (string, error) { return "gke-id", nil }
	detectEKS = func(context.Context, *log.Logger, string) (string, error) { return "eks-id", nil }
	detectAKS = func(*log.Logger, string) (string, error) { return "aks-id", nil }

	tests := []struct {
		name         string
		providerID   string
		wantID       string
		wantProvider Provider
		wantErr      bool
	}{
		{
			name:         "routesGKE",
			providerID:   "gce://my-project/us-central1-a/gke-node",
			wantID:       "gke-id",
			wantProvider: ProviderGKE,
		},
		{
			name:         "routesEKS",
			providerID:   "aws:///us-east-1a/i-0abc",
			wantID:       "eks-id",
			wantProvider: ProviderEKS,
		},
		{
			name:         "routesAKS",
			providerID:   "azure:///subscriptions/sub/resourceGroups/mc_rg_cluster_westus2/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0",
			wantID:       "aks-id",
			wantProvider: ProviderAKS,
		},
		{
			name:         "unknownProviderErrors",
			providerID:   "openstack:///abc",
			wantProvider: ProviderUnknown,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8s := fake.NewSimpleClientset(&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec:       corev1.NodeSpec{ProviderID: tt.providerID},
			})

			gotID, gotProvider, err := DetectClusterId(context.Background(), logutil.Discard, k8s, nodeName)
			if gotProvider != tt.wantProvider {
				t.Errorf("DetectClusterId() provider = %q, want %q", gotProvider, tt.wantProvider)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DetectClusterId() expected error, got id %q", gotID)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectClusterId() unexpected error = %v", err)
			}
			if gotID != tt.wantID {
				t.Errorf("DetectClusterId() id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}
