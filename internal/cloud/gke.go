package cloud

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// detectGKE assembles the GKE cluster link /projects/<projectId>/locations/<location>/clusters/<clusterName>.
// projectId comes from the node's providerID (falling back to the metadata server);
// clusterName and location come from the GKE metadata server instance attributes.
// cluster-location is a zone for zonal clusters and a region for regional clusters;
// GKE's locations path accepts both, so no zonal/regional branching is needed.
var detectGKE = func(ctx context.Context, providerID string) (string, error) {
	name, err := gkeMetadata(ctx, "/instance/attributes/cluster-name")
	if err != nil {
		return "", fmt.Errorf("querying GKE cluster-name: %w", err)
	}
	location, err := gkeMetadata(ctx, "/instance/attributes/cluster-location")
	if err != nil {
		return "", fmt.Errorf("querying GKE cluster-location: %w", err)
	}

	projectID, _, _ := parseGCEProviderID(providerID)
	if projectID == "" {
		projectID, err = gkeMetadata(ctx, "/project/project-id")
		if err != nil {
			return "", fmt.Errorf("querying GKE project-id: %w", err)
		}
	}

	if projectID == "" || location == "" || name == "" {
		return "", fmt.Errorf("incomplete GKE metadata: project=%q location=%q cluster=%q", projectID, location, name)
	}
	return fmt.Sprintf("/projects/%s/locations/%s/clusters/%s", projectID, location, name), nil
}

// parseGCEProviderID extracts project, zone and instance from providerID.
func parseGCEProviderID(providerID string) (projectID, zone, instance string) {
	re := regexp.MustCompile(`^gce://([a-zA-Z0-9._\-()]+)/([a-zA-Z0-9._\-()]+)/([a-zA-Z0-9._\-()]+)$`)
	matches := re.FindStringSubmatch(providerID)
	if matches == nil {
		return "", "", ""
	}
	return matches[1], matches[2], matches[3]
}

// gkeMetadata performs a GET against the GCP metadata server and returns the trimmed body.
func gkeMetadata(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gkeMetadataBaseURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("building GKE metadata request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	body, err := doMetadataGet(req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(body), nil
}
