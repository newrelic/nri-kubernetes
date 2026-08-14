package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_parseGCEProviderID(t *testing.T) {
	tests := []struct {
		name         string
		providerID   string
		wantProject  string
		wantZone     string
		wantInstance string
	}{
		{
			name:         "parseCorrectly", //nolint:goconst
			providerID:   "gce://my-project/us-central1-a/gke-cluster-default-pool-abc",
			wantProject:  "my-project",
			wantZone:     "us-central1-a",
			wantInstance: "gke-cluster-default-pool-abc",
		},
		{
			name:         "emptyOnWrongPrefix",
			providerID:   "aws:///us-east-2a/i-0abc",
			wantZone:     "",
			wantInstance: "",
		},
		{
			name:         "emptyOnMissingComponents",
			providerID:   "gce://my-project/us-central1-a",
			wantZone:     "",
			wantInstance: "",
		},
		{
			name:         "emptyOnExtraComponents",
			providerID:   "gce://my-project/us-central1-a/instance/extra",
			wantZone:     "",
			wantInstance: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotProject, gotZone, gotInstance := parseGCEProviderID(tt.providerID)
			if gotProject != tt.wantProject || gotZone != tt.wantZone || gotInstance != tt.wantInstance {
				t.Errorf("parseGCEProviderID() = %q, %q, %q; want %q, %q, %q",
					gotProject, gotZone, gotInstance, tt.wantProject, tt.wantZone, tt.wantInstance)
			}
		})
	}
}

func Test_detectGKE(t *testing.T) {
	tests := []struct {
		name        string
		providerID  string
		clusterName string
		location    string
		wantID      string
	}{
		{
			name:        "zonalCluster",
			providerID:  "gce://my-project/us-central1-a/gke-node",
			clusterName: "my-zonal-cluster",
			location:    "us-central1-a",
			wantID:      "/projects/my-project/locations/us-central1-a/clusters/my-zonal-cluster",
		},
		{
			name:        "regionalCluster",
			providerID:  "gce://my-project/us-central1-a/gke-node",
			clusterName: "my-regional-cluster",
			location:    "us-central1",
			wantID:      "/projects/my-project/locations/us-central1/clusters/my-regional-cluster",
		},
		{
			name:        "projectFromMetadataFallback",
			providerID:  "", // no project in providerID -> falls back to metadata project-id
			clusterName: "my-cluster",
			location:    "us-central1",
			wantID:      "/projects/meta-project/locations/us-central1/clusters/my-cluster",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Metadata-Flavor") != "Google" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				switch r.URL.Path {
				case "/instance/attributes/cluster-name":
					_, _ = w.Write([]byte(tt.clusterName))
				case "/instance/attributes/cluster-location":
					_, _ = w.Write([]byte(tt.location))
				case "/project/project-id":
					_, _ = w.Write([]byte("meta-project"))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			old := gkeMetadataBaseURL
			gkeMetadataBaseURL = srv.URL
			defer func() { gkeMetadataBaseURL = old }()

			got, err := detectGKE(context.Background(), tt.providerID)
			if err != nil {
				t.Fatalf("detectGKE() unexpected error = %v", err)
			}
			if got != tt.wantID {
				t.Errorf("detectGKE() = %q, want %q", got, tt.wantID)
			}
		})
	}
}
