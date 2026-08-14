package cloud

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeDetector is a stub providerDetector used to test routing in isolation.
type fakeDetector struct {
	id  string
	err error
}

func (f fakeDetector) detect(context.Context, string) (string, error) { return f.id, f.err }

func Test_DetectClusterID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerID   string
		wantID       string
		wantProvider Provider
		wantErr      bool
	}{
		{
			name:         "routesGKE",
			providerID:   "gce://my-project/us-west1-a/gke-node",
			wantID:       "gke-id",
			wantProvider: ProviderGKE,
		},
		{
			name:         "routesEKS",
			providerID:   "aws:///us-west-1a/i-0abc",
			wantID:       "eks-id",
			wantProvider: ProviderEKS,
		},
		{
			name:         "routesAKS",
			providerID:   "azure:///subscriptions/sub/resourceGroups/mc_rg_cluster_westus1/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0",
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
			t.Parallel()
			detector := &Detector{
				gke: fakeDetector{id: "gke-id"},
				eks: fakeDetector{id: "eks-id"},
				aks: fakeDetector{id: "aks-id"},
			}
			k8s := fake.NewSimpleClientset(&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Spec:       corev1.NodeSpec{ProviderID: tt.providerID},
			})

			gotID, gotProvider, err := detector.DetectClusterID(context.Background(), k8s, "node-1")
			if gotProvider != tt.wantProvider {
				t.Errorf("DetectClusterID() provider = %q, want %q", gotProvider, tt.wantProvider)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DetectClusterID() expected error, got id %q", gotID)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectClusterID() unexpected error = %v", err)
			}
			if gotID != tt.wantID {
				t.Errorf("DetectClusterID() id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func Test_DetectClusterID_NodeNotFound(t *testing.T) {
	t.Parallel()
	detector := NewDetector(nil)
	k8s := fake.NewSimpleClientset() // no nodes

	if _, _, err := detector.DetectClusterID(context.Background(), k8s, "missing-node"); err == nil {
		t.Fatal("DetectClusterID() expected error for missing node, got nil")
	}
}
