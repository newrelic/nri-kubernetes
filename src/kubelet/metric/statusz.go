package metric

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/data"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	// StatuszPath is the path where kubelet serves its status
	StatuszPath = "/statusz"
)

// KubeletStatuszFetcher fetches kubelet status information from /statusz endpoint.
// Supports both modern JSON format (Kubernetes 1.35+) and legacy text format (older versions).
type KubeletStatuszFetcher struct {
	httpClient client.HTTPGetter
	nodeName   string
}

// ComponentStatus represents the health status of a kubelet component (PLEG, RuntimeReady, etc.)
type ComponentStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "healthy", "unhealthy", "unknown"
}

// StatuszResponse represents the structured status response from modern kubelet versions
type StatuszResponse struct {
	HealthStatus      string            `json:"healthStatus"`      // "healthy", "unhealthy"
	ComponentStatuses []ComponentStatus `json:"componentStatuses"` // Individual component health
}

// NewKubeletStatuszFetcher creates a new fetcher for kubelet status
func NewKubeletStatuszFetcher(httpClient client.HTTPGetter, nodeName string) *KubeletStatuszFetcher {
	return &KubeletStatuszFetcher{
		httpClient: httpClient,
		nodeName:   nodeName,
	}
}

// Fetch retrieves kubelet status from /statusz endpoint
func (f *KubeletStatuszFetcher) Fetch() (definition.RawGroups, error) {
	var resp *http.Response
	var err error

	// Try to use content negotiation if the client supports it
	// Request JSON format first (preferred for structured component health data)
	if clientWithAccept, ok := f.httpClient.(client.HTTPGetterWithAccept); ok {
		resp, err = clientWithAccept.GetWithAccept(StatuszPath, "application/json, text/plain;q=0.9")
	} else {
		resp, err = f.httpClient.Get(StatuszPath)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching kubelet statusz: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading statusz response: %w", err)
	}

	// Check Content-Type header to determine parsing strategy
	contentType := resp.Header.Get("Content-Type")

	// Parse the response based on Content-Type, with fallback to auto-detection
	var statusz *StatuszResponse
	var isJSON bool
	if strings.HasPrefix(contentType, "application/json") {
		// Server explicitly returned JSON
		statusz, isJSON = parseStatusz(body)
	} else if strings.HasPrefix(contentType, "text/plain") {
		// Server explicitly returned text - skip JSON parsing attempt
		statusz, isJSON = parseStatuszText(body)
	} else {
		// Unknown or missing Content-Type - try both (JSON first)
		statusz, isJSON = parseStatusz(body)
	}

	// Generate fingerprint for change detection
	fingerprint := generateFingerprint(body)

	// Build the raw metrics
	g := definition.RawGroups{
		"node": {
			f.nodeName: make(definition.RawMetrics),
		},
	}

	nodeMetrics := g["node"][f.nodeName]

	// Add basic status information
	nodeMetrics["kubeletStatuszFingerprint"] = fingerprint
	nodeMetrics["kubeletStatuszResponseFormat"] = map[bool]string{true: "json", false: "text"}[isJSON]
	nodeMetrics["kubeletStatuszHealthy"] = statusz.HealthStatus == "healthy"
	nodeMetrics["kubeletStatuszOverallStatus"] = statusz.HealthStatus

	// Add component-level status if available (Kubernetes 1.35+)
	if len(statusz.ComponentStatuses) > 0 {
		componentHealth := make(map[string]interface{})
		for _, comp := range statusz.ComponentStatuses {
			// Store individual component status
			metricName := fmt.Sprintf("kubeletStatuszComponent_%s", comp.Name)
			nodeMetrics[metricName] = comp.Status

			// Track healthy/unhealthy counts
			componentHealth[comp.Name] = comp.Status
		}

		// Store component health as JSON for queryability
		if healthJSON, err := json.Marshal(componentHealth); err == nil {
			nodeMetrics["kubeletStatuszComponentsJSON"] = string(healthJSON)
		}

		// Count unhealthy components - only valid for JSON format where
		// component statuses are actual health values ("healthy", "unhealthy", "unknown")
		// For text format, the "status" values are metadata like version strings
		if isJSON {
			unhealthyCount := 0
			for _, comp := range statusz.ComponentStatuses {
				if comp.Status != "healthy" {
					unhealthyCount++
				}
			}
			nodeMetrics["kubeletStatuszUnhealthyComponents"] = unhealthyCount
		}
	}

	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform)
	// This needs to be a map[string]interface{}, not a JSON string
	statuszDiagnostics := make(map[string]interface{})
	for k, v := range nodeMetrics {
		if len(k) > 14 && k[:14] == "kubeletStatusz" {
			statuszDiagnostics[k[14:]] = v
		} else {
			statuszDiagnostics[k] = v
		}
	}
	nodeMetrics["kubeletStatuszDiagnostics"] = statuszDiagnostics

	return g, nil
}

// parseStatusz attempts to parse statusz response as JSON (modern format) or text (legacy/verbose format)
func parseStatusz(body []byte) (*StatuszResponse, bool) {
	// Try parsing as JSON first
	var jsonResponse StatuszResponse
	if err := json.Unmarshal(body, &jsonResponse); err == nil && jsonResponse.HealthStatus != "" {
		// Successfully parsed as JSON
		return &jsonResponse, true
	}

	// Fall back to text parsing
	return parseStatuszText(body)
}

// parseStatuszText parses statusz response as text (verbose or legacy format)
func parseStatuszText(body []byte) (*StatuszResponse, bool) {
	text := strings.TrimSpace(string(body))
	status := StatuszResponse{}

	// Check if this is the verbose format (contains "kubelet statusz" header)
	// The kubelet output format varies: some versions use "Key= Value" with equals,
	// others use "Key:  Value" with colon. We need to handle both.
	if strings.Contains(text, "kubelet statusz") {
		// Verbose format from /statusz endpoint with feature gate enabled
		// Format varies by version:
		//   kubelet statusz
		//   Warning: ...
		//   Started= Wed Feb 18 16:09:37 UTC 2026   (equals format)
		//   Started:  Wed Feb 18 16:09:37 UTC 2026  (colon format)
		//   Up= 0 hr 00 min 21 sec
		//   Go version= go1.25.5
		//   Binary version= 1.35.0
		//   Emulation version= 1.35
		//   Paths= /configz /debug /flagz /healthz /metrics
		//
		// NOTE: Text format contains metadata (version, uptime, etc.), NOT real component health.
		// We do NOT create ComponentStatuses for text format - only JSON format has real health data.
		status.HealthStatus = "healthy" // If we can read verbose output, kubelet is responsive

		return &status, false
	}

	// Legacy simple format: single word like "ok", "healthy", "unhealthy"
	switch strings.ToLower(text) {
	case "ok", "healthy":
		status.HealthStatus = "healthy"
	case "unhealthy", "error":
		status.HealthStatus = "unhealthy"
	default:
		if text == "" {
			status.HealthStatus = "unknown"
		} else {
			// Keep original text for unrecognized status
			status.HealthStatus = text
		}
	}

	return &status, false
}

// generateFingerprint creates a SHA256 hash of the statusz response for change detection
func generateFingerprint(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// KubeletStatuszFetchFunc creates a FetchFunc that fetches kubelet status
func KubeletStatuszFetchFunc(httpClient client.HTTPGetter, nodeName string) data.FetchFunc {
	fetcher := NewKubeletStatuszFetcher(httpClient, nodeName)
	return fetcher.Fetch
}
