package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// detectGKE reads the cluster name from the GCP metadata server instance attribute.
func detectGKE(ctx context.Context) (string, error) {
	url := gkeMetadataBaseURL + "/instance/attributes/cluster-name"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building GKE metadata request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	name, err := doMetadataGet(req)
	if err != nil {
		return "", fmt.Errorf("querying GKE metadata: %w", err)
	}
	return strings.TrimSpace(name), nil
}
