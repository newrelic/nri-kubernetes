package cloud

import (
	"context"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
)

func Test_parseAzureProviderID(t *testing.T) {
	t.Parallel()
	type args struct {
		providerID string
	}
	tests := []struct {
		name               string
		args               args
		wantSubscriptionID string
		wantResourceGroup  string
	}{
		{
			name: "parseCorrectly", //nolint:goconst
			args: args{
				"azure:///subscriptions/f038e9fc-3c25-459e-9bea-879af3be645e/resourceGroups/mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2/providers/Microsoft.Compute/virtualMachineScaleSets/aks-linux-38321548-vmss/virtualMachines/0",
			},
			wantSubscriptionID: "f038e9fc-3c25-459e-9bea-879af3be645e",
			wantResourceGroup:  "mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2",
		},
		{
			name: "emptyResponseOnInvalidFormat",
			args: args{
				"azure:///subs/f038e9fc-3c25-459e-9bea-879af3be645e/rgs/mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2/providers/Microsoft.Compute/virtualMachineScaleSets/aks-linux-38321548-vmss/virtualMachines/0",
			},
			wantSubscriptionID: "",
			wantResourceGroup:  "",
		},
		{
			name: "emptyResponseOnMissingComponents",
			args: args{
				"azure:///subscriptions/f038e9fc-3c25-459e-9bea-879af3be645e/resourceGroups/mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2",
			},
			wantSubscriptionID: "",
			wantResourceGroup:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotSubscriptionID, gotResourceGroup := parseAzureProviderID(tt.args.providerID)
			if gotSubscriptionID != tt.wantSubscriptionID {
				t.Errorf("parseAzureProviderID() gotSubscriptionID = %v, want %v", gotSubscriptionID, tt.wantSubscriptionID)
			}
			if gotResourceGroup != tt.wantResourceGroup {
				t.Errorf("parseAzureProviderID() gotResourceGroup = %v, want %v", gotResourceGroup, tt.wantResourceGroup)
			}
		})
	}
}

func Test_detectAKS(t *testing.T) {
	t.Parallel()
	type args struct {
		providerID string
	}
	tests := []struct {
		name    string
		args    args
		wantID  string
		wantErr bool
	}{
		{
			name: "assemblesARMID",
			args: args{
				"azure:///subscriptions/f038e9fc-3c25-459e-9bea-879af3be645e/resourceGroups/mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2/providers/Microsoft.Compute/virtualMachineScaleSets/aks-linux-38321548-vmss/virtualMachines/0",
			},
			wantID: "/subscriptions/f038e9fc-3c25-459e-9bea-879af3be645e/resourceGroups/k8sagentsusersresourcegroup/providers/Microsoft.ContainerService/managedClusters/ttrojan-aks-tsty",
		},
		{
			name: "errorOnCustomNodeResourceGroup",
			args: args{
				"azure:///subscriptions/f038e9fc-3c25-459e-9bea-879af3be645e/resourceGroups/custom-node-rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-linux-38321548-vmss/virtualMachines/0",
			},
			wantErr: true,
		},
		{
			name: "errorOnInvalidProviderID",
			args: args{
				"azure:///subs/f038e9fc-3c25-459e-9bea-879af3be645e/rgs/mc_rg_cluster_westus2",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, err := aksDetector{logger: logutil.Discard}.detect(context.Background(), tt.args.providerID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("detectAKS() expected error, got id %q", gotID)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectAKS() unexpected error = %v", err)
			}
			if gotID != tt.wantID {
				t.Errorf("detectAKS() = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func Test_parseAKSNodeResourceGroup(t *testing.T) {
	t.Parallel()
	type args struct {
		nodeRG string
	}
	tests := []struct {
		name              string
		args              args
		wantResourceGroup string
		wantCluster       string
	}{
		{
			name: "parseCorrectly",
			args: args{
				"mc_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2",
			},
			wantResourceGroup: "k8sagentsusersresourcegroup",
			wantCluster:       "ttrojan-aks-tsty",
		},
		{
			name: "parseCorrectlyCaseInsensitive",
			args: args{
				"mC_k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2",
			},
			wantResourceGroup: "k8sagentsusersresourcegroup",
			wantCluster:       "ttrojan-aks-tsty",
		},
		{
			name: "emptyResponseOnMissingComponents",
			args: args{
				"k8sagentsusersresourcegroup_ttrojan-aks-tsty_westus2",
			},
			wantResourceGroup: "",
			wantCluster:       "",
		},
		{
			name: "emptyResponseOnUnsupportedFormat",
			args: args{
				"mc_k8sagentsusersresourcegroup_ttrojan_aks-tsty_westus2",
			},
			wantResourceGroup: "",
			wantCluster:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotResourceGroup, gotCluster := parseAKSNodeResourceGroup(tt.args.nodeRG)
			if gotResourceGroup != tt.wantResourceGroup {
				t.Errorf("parseAKSNodeResourceGroup() gotResourceGroup = %v, want %v", gotResourceGroup, tt.wantResourceGroup)
			}
			if gotCluster != tt.wantCluster {
				t.Errorf("parseAKSNodeResourceGroup() gotCluster = %v, want %v", gotCluster, tt.wantCluster)
			}
		})
	}
}
