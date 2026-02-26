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
	// StatuszPath is the path where kubelet serves its status.
	StatuszPath = "/statusz"
)

// KubeletStatuszFetcher fetches kubelet status information from /statusz endpoint.
// Supports both modern JSON format (Kubernetes 1.35+) and legacy text format (older versions).
type KubeletStatuszFetcher struct {
	httpClient client.HTTPGetter
	nodeName   string
}

// ComponentStatus represents the health status of a kubelet component (PLEG, RuntimeReady, etc.).
type ComponentStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "healthy", "unhealthy", "unknown"
}

// StatuszResponse represents the structured status response from modern kubelet versions.
type StatuszResponse struct {
	HealthStatus      string            `json:"healthStatus"`      // "healthy", "unhealthy"
	ComponentStatuses []ComponentStatus `json:"componentStatuses"` // Individual component health
}

// NewKubeletStatuszFetcher creates a new fetcher for kubelet status.
func NewKubeletStatuszFetcher(httpClient client.HTTPGetter, nodeName string) *KubeletStatuszFetcher {
	return &KubeletStatuszFetcher{
		httpClient: httpClient,
		nodeName:   nodeName,
	}
}

// Fetch retrieves kubelet status from /statusz endpoint.
func (f *KubeletStatuszFetcher) Fetch() (definition.RawGroups, error) {
	resp, err := f.fetchStatusz()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading statusz response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	statusz, isJSON := f.parseResponse(contentType, body)
	fingerprint := generateFingerprint(body)

	return f.buildRawGroups(statusz, isJSON, fingerprint), nil
}

func (f *KubeletStatuszFetcher) fetchStatusz() (*http.Response, error) {
	var resp *http.Response
	var err error

	// Try to use content negotiation if the client supports it.
	// Request JSON format first (preferred for structured component health data).
	if clientWithAccept, ok := f.httpClient.(client.HTTPGetterWithAccept); ok {
		resp, err = clientWithAccept.GetWithAccept(StatuszPath, "application/json, text/plain;q=0.9")
	} else {
		resp, err = f.httpClient.Get(StatuszPath)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching kubelet statusz: %w", err)
	}

	return resp, nil
}

func (f *KubeletStatuszFetcher) parseResponse(contentType string, body []byte) (*StatuszResponse, bool) {
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		// Server explicitly returned JSON.
		return parseStatusz(body)
	case strings.HasPrefix(contentType, "text/plain"):
		// Server explicitly returned text - skip JSON parsing attempt.
		return parseStatuszText(body)
	default:
		// Unknown or missing Content-Type - try both (JSON first).
		return parseStatusz(body)
	}
}

func (f *KubeletStatuszFetcher) buildRawGroups(statusz *StatuszResponse, isJSON bool, fingerprint string) definition.RawGroups {
	g := definition.RawGroups{
		"node": {
			f.nodeName: make(definition.RawMetrics),
		},
	}

	nodeMetrics := g["node"][f.nodeName]

	// Add basic status information.
	nodeMetrics["kubeletStatuszFingerprint"] = fingerprint
	responseFormat := "text"
	if isJSON {
		responseFormat = "json"
	}
	nodeMetrics["kubeletStatuszResponseFormat"] = responseFormat
	nodeMetrics["kubeletStatuszHealthy"] = statusz.HealthStatus == statusHealthy
	nodeMetrics["kubeletStatuszOverallStatus"] = statusz.HealthStatus

	// Add component-level status if available (Kubernetes 1.35+).
	if len(statusz.ComponentStatuses) > 0 {
		f.addComponentMetrics(statusz, isJSON, nodeMetrics)
	}

	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform).
	f.addDiagnosticsMap(nodeMetrics)

	return g
}

func (f *KubeletStatuszFetcher) addComponentMetrics(statusz *StatuszResponse, isJSON bool, nodeMetrics definition.RawMetrics) {
	componentHealth := make(map[string]interface{})
	for _, comp := range statusz.ComponentStatuses {
		// Store individual component status.
		metricName := fmt.Sprintf("kubeletStatuszComponent_%s", comp.Name)
		nodeMetrics[metricName] = comp.Status

		// Track healthy/unhealthy counts.
		componentHealth[comp.Name] = comp.Status
	}

	// Store component health as JSON for queryability.
	if healthJSON, err := json.Marshal(componentHealth); err == nil {
		nodeMetrics["kubeletStatuszComponentsJSON"] = string(healthJSON)
	}

	// Count unhealthy components - only valid for JSON format where
	// component statuses are actual health values ("healthy", "unhealthy", "unknown").
	// For text format, the "status" values are metadata like version strings.
	if isJSON {
		unhealthyCount := 0
		for _, comp := range statusz.ComponentStatuses {
			if comp.Status != statusHealthy {
				unhealthyCount++
			}
		}
		nodeMetrics["kubeletStatuszUnhealthyComponents"] = unhealthyCount
	}
}

func (f *KubeletStatuszFetcher) addDiagnosticsMap(nodeMetrics definition.RawMetrics) {
	statuszDiagnostics := make(map[string]interface{})
	const statuszPrefixLen = 14
	for k, v := range nodeMetrics {
		if len(k) > statuszPrefixLen && k[:statuszPrefixLen] == "kubeletStatusz" {
			statuszDiagnostics[k[statuszPrefixLen:]] = v
		} else {
			statuszDiagnostics[k] = v
		}
	}
	nodeMetrics["kubeletStatuszDiagnostics"] = statuszDiagnostics
}

// parseStatusz attempts to parse statusz response as JSON (modern format) or text (legacy/verbose format).
func parseStatusz(body []byte) (*StatuszResponse, bool) {
	// Try parsing as JSON first.
	var jsonResponse StatuszResponse
	if err := json.Unmarshal(body, &jsonResponse); err == nil && jsonResponse.HealthStatus != "" {
		// Successfully parsed as JSON.
		return &jsonResponse, true
	}

	// Fall back to text parsing.
	return parseStatuszText(body)
}

// parseStatuszText parses statusz response as text (verbose or legacy format).
func parseStatuszText(body []byte) (*StatuszResponse, bool) {
	text := strings.TrimSpace(string(body))
	status := StatuszResponse{}

	// Check if this is the verbose format (contains "kubelet statusz" header).
	// The kubelet output format varies: some versions use "Key= Value" with equals,
	// others use "Key:  Value" with colon. We need to handle both.
	if strings.Contains(text, "kubelet statusz") {
		// Verbose format from /statusz endpoint with feature gate enabled.
		// NOTE: Text format contains metadata (version, uptime, etc.), NOT real component health.
		// We do NOT create ComponentStatuses for text format - only JSON format has real health data.
		status.HealthStatus = statusHealthy // If we can read verbose output, kubelet is responsive.
		return &status, false
	}

	// Legacy simple format: single word like "ok", "healthy", "unhealthy".
	status.HealthStatus = parseSimpleStatus(strings.ToLower(text))

	return &status, false
}

func parseSimpleStatus(text string) string {
	switch text {
	case "ok", statusHealthy:
		return statusHealthy
	case statusUnhealthy, "error":
		return statusUnhealthy
	default:
		if text == "" {
			return "unknown"
		}
		// Keep original text for unrecognized status.
		return text
	}
}

// generateFingerprint creates a SHA256 hash of the statusz response for change detection.
func generateFingerprint(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// KubeletStatuszFetchFunc creates a FetchFunc that fetches kubelet status.
func KubeletStatuszFetchFunc(httpClient client.HTTPGetter, nodeName string) data.FetchFunc {
	fetcher := NewKubeletStatuszFetcher(httpClient, nodeName)
	return fetcher.Fetch
}
