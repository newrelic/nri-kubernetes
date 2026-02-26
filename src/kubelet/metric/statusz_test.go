package metric

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPGetterStatusz is a mock HTTP client for testing statusz fetcher.
type mockHTTPGetterStatusz struct {
	response string
	isJSON   bool
}

func (m *mockHTTPGetterStatusz) Get(_ string) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(m.response)),
	}, nil
}

func (m *mockHTTPGetterStatusz) GetURI(_ url.URL) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(m.response)),
	}, nil
}

func TestKubeletStatuszFetcher_JSONFormat_Kubernetes135(t *testing.T) {
	t.Parallel()
	// Modern Kubernetes 1.35+ response with component-level status.
	jsonResponse := `{
  "healthStatus": "healthy",
  "componentStatuses": [
    {
      "name": "PLEG",
      "status": "healthy"
    },
    {
      "name": "RuntimeReady",
      "status": "healthy"
    },
    {
      "name": "NetworkReady",
      "status": "healthy"
    },
    {
      "name": "StoreReady",
      "status": "healthy"
    }
  ]
}`

	mockClient := &mockHTTPGetterStatusz{
		response: jsonResponse,
		isJSON:   true,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)
	require.NotNil(t, rawGroups)

	// Verify node group exists.
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok, "node group should exist")

	// Verify test-node exists.
	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok, "test-node should exist in node group")

	// Verify basic status fields.
	assert.Equal(t, "json", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, true, nodeMetrics["kubeletStatuszHealthy"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszOverallStatus"])

	// Verify fingerprint exists.
	fingerprint, ok := nodeMetrics["kubeletStatuszFingerprint"]
	assert.True(t, ok, "fingerprint should exist")
	assert.NotEmpty(t, fingerprint, "fingerprint should not be empty")

	// Verify component-level status (Kubernetes 1.35+ feature).
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszComponent_PLEG"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszComponent_RuntimeReady"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszComponent_NetworkReady"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszComponent_StoreReady"])

	// Verify unhealthy component count.
	assert.Equal(t, 0, nodeMetrics["kubeletStatuszUnhealthyComponents"])

	// Verify component JSON exists.
	componentsJSON, ok := nodeMetrics["kubeletStatuszComponentsJSON"]
	assert.True(t, ok, "components JSON should exist")
	assert.NotEmpty(t, componentsJSON, "components JSON should not be empty")
}

func TestKubeletStatuszFetcher_JSONFormat_Unhealthy(t *testing.T) {
	t.Parallel()
	// Unhealthy kubelet with mixed component statuses.
	jsonResponse := `{
  "healthStatus": "unhealthy",
  "componentStatuses": [
    {
      "name": "PLEG",
      "status": "unhealthy"
    },
    {
      "name": "RuntimeReady",
      "status": "healthy"
    },
    {
      "name": "NetworkReady",
      "status": "unknown"
    }
  ]
}`

	mockClient := &mockHTTPGetterStatusz{
		response: jsonResponse,
		isJSON:   true,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)
	require.NotNil(t, rawGroups)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Verify unhealthy overall status.
	assert.Equal(t, false, nodeMetrics["kubeletStatuszHealthy"])
	assert.Equal(t, "unhealthy", nodeMetrics["kubeletStatuszOverallStatus"])

	// Verify component statuses.
	assert.Equal(t, "unhealthy", nodeMetrics["kubeletStatuszComponent_PLEG"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszComponent_RuntimeReady"])
	assert.Equal(t, "unknown", nodeMetrics["kubeletStatuszComponent_NetworkReady"])

	// Verify unhealthy count (PLEG=unhealthy, NetworkReady=unknown).
	assert.Equal(t, 2, nodeMetrics["kubeletStatuszUnhealthyComponents"])
}

func TestKubeletStatuszFetcher_TextFormat_Legacy_OK(t *testing.T) {
	t.Parallel()
	// Older Kubernetes versions return plain text.
	textResponse := "ok"

	mockClient := &mockHTTPGetterStatusz{
		response: textResponse,
		isJSON:   false,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)
	require.NotNil(t, rawGroups)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Verify text format detection.
	assert.Equal(t, "text", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, true, nodeMetrics["kubeletStatuszHealthy"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszOverallStatus"])

	// No component-level status in legacy format.
	_, hasComponents := nodeMetrics["kubeletStatuszComponentsJSON"]
	assert.False(t, hasComponents, "legacy format should not have component JSON")
}

func TestKubeletStatuszFetcher_TextFormat_Legacy_Healthy(t *testing.T) {
	t.Parallel()
	// Some versions return "healthy" as text.
	textResponse := "healthy"

	mockClient := &mockHTTPGetterStatusz{
		response: textResponse,
		isJSON:   false,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	assert.Equal(t, "text", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, true, nodeMetrics["kubeletStatuszHealthy"])
	assert.Equal(t, "healthy", nodeMetrics["kubeletStatuszOverallStatus"])
}

func TestKubeletStatuszFetcher_TextFormat_Legacy_Unhealthy(t *testing.T) {
	t.Parallel()
	// Error state in text format.
	textResponse := "unhealthy"

	mockClient := &mockHTTPGetterStatusz{
		response: textResponse,
		isJSON:   false,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	assert.Equal(t, "text", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, false, nodeMetrics["kubeletStatuszHealthy"])
	assert.Equal(t, "unhealthy", nodeMetrics["kubeletStatuszOverallStatus"])
}

func TestKubeletStatuszFetcher_TextFormat_Unknown(t *testing.T) {
	t.Parallel()
	// Unexpected response.
	textResponse := "some unexpected status"

	mockClient := &mockHTTPGetterStatusz{
		response: textResponse,
		isJSON:   false,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Should keep the original text but not mark as healthy.
	assert.Equal(t, "text", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, "some unexpected status", nodeMetrics["kubeletStatuszOverallStatus"])
	assert.Equal(t, false, nodeMetrics["kubeletStatuszHealthy"])
}

func TestKubeletStatuszFetcher_EmptyResponse(t *testing.T) {
	t.Parallel()
	mockClient := &mockHTTPGetterStatusz{
		response: "",
		isJSON:   false,
	}

	fetcher := NewKubeletStatuszFetcher(mockClient, "test-node")
	rawGroups, err := fetcher.Fetch()

	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	assert.Equal(t, "text", nodeMetrics["kubeletStatuszResponseFormat"])
	assert.Equal(t, "unknown", nodeMetrics["kubeletStatuszOverallStatus"])
	assert.Equal(t, false, nodeMetrics["kubeletStatuszHealthy"])
}

func TestParseStatusz_JSONFormat(t *testing.T) {
	t.Parallel()
	jsonData := []byte(`{
		"healthStatus": "healthy",
		"componentStatuses": [
			{"name": "PLEG", "status": "healthy"}
		]
	}`)

	result, isJSON := parseStatusz(jsonData)

	assert.True(t, isJSON, "should detect JSON format")
	assert.Equal(t, "healthy", result.HealthStatus)
	assert.Len(t, result.ComponentStatuses, 1)
	assert.Equal(t, "PLEG", result.ComponentStatuses[0].Name)
	assert.Equal(t, "healthy", result.ComponentStatuses[0].Status)
}

func TestParseStatusz_TextFormat_OK(t *testing.T) {
	t.Parallel()
	textData := []byte("ok")

	result, isJSON := parseStatusz(textData)

	assert.False(t, isJSON, "should detect text format")
	assert.Equal(t, "healthy", result.HealthStatus) // "ok" normalized to "healthy".
	assert.Len(t, result.ComponentStatuses, 0)
}

func TestParseStatusz_TextFormat_Healthy(t *testing.T) {
	t.Parallel()
	textData := []byte("healthy")

	result, isJSON := parseStatusz(textData)

	assert.False(t, isJSON, "should detect text format")
	assert.Equal(t, "healthy", result.HealthStatus)
}

func TestParseStatusz_TextFormat_Unhealthy(t *testing.T) {
	t.Parallel()
	textData := []byte("unhealthy")

	result, isJSON := parseStatusz(textData)

	assert.False(t, isJSON, "should detect text format")
	assert.Equal(t, "unhealthy", result.HealthStatus)
}

func TestParseStatusz_TextFormat_WithWhitespace(t *testing.T) {
	t.Parallel()
	textData := []byte("  ok  \n")

	result, isJSON := parseStatusz(textData)

	assert.False(t, isJSON, "should detect text format")
	assert.Equal(t, "healthy", result.HealthStatus) // Trimmed and normalized.
}

func TestGenerateFingerprint(t *testing.T) {
	t.Parallel()
	data1 := []byte("test data")
	data2 := []byte("test data")
	data3 := []byte("different data")

	fp1 := generateFingerprint(data1)
	fp2 := generateFingerprint(data2)
	fp3 := generateFingerprint(data3)

	// Same data should produce same fingerprint.
	assert.Equal(t, fp1, fp2)

	// Different data should produce different fingerprint.
	assert.NotEqual(t, fp1, fp3)

	// Fingerprint should be hex string.
	assert.Len(t, fp1, 64) // SHA256 produces 64 hex characters.
}

func TestKubeletStatuszFetchFunc(t *testing.T) {
	t.Parallel()
	mockClient := &mockHTTPGetterStatusz{
		response: "ok",
		isJSON:   false,
	}

	fetchFunc := KubeletStatuszFetchFunc(mockClient, "test-node")
	rawGroups, err := fetchFunc()

	require.NoError(t, err)
	require.NotNil(t, rawGroups)

	nodeMetrics := rawGroups["node"]["test-node"]
	assert.Equal(t, true, nodeMetrics["kubeletStatuszHealthy"])
}

func TestParseStatusz_VerboseTextFormat_K135_EqualsFormat(t *testing.T) {
	t.Parallel()
	// This is the exact format returned by Kubernetes 1.35 kubelet with ComponentStatusz feature gate.
	// Note: The format uses equals sign (=) as separator with space after equals.
	// Text format contains metadata (version, uptime), NOT real component health.
	// ComponentStatuses should only be populated from JSON format.
	verboseResponse := "\nkubelet statusz\nWarning: This endpoint is not meant to be machine parseable, has no formatting compatibility guarantees and is for debugging purposes only.\n\nStarted= Wed Feb 18 22:09:26 UTC 2026\nUp= 22 hr 13 min 30 sec\nGo version= go1.25.5\nBinary version= 1.35.0\nEmulation version= 1.35\nPaths= /configz /debug /flagz /healthz /metrics\n"

	result, isJSON := parseStatusz([]byte(verboseResponse))

	assert.False(t, isJSON, "should detect text format (not JSON)")
	assert.Equal(t, "healthy", result.HealthStatus)

	// Text format should NOT create ComponentStatuses - only JSON format has real health data.
	assert.Empty(t, result.ComponentStatuses, "text format should not create synthetic component statuses")
}

func TestParseStatusz_VerboseTextFormat_K135_ColonFormat(t *testing.T) {
	t.Parallel()
	// Alternative format with colons (some versions might use this).
	// Text format contains metadata (version, uptime), NOT real component health.
	// ComponentStatuses should only be populated from JSON format.
	verboseResponse := `
kubelet statusz
Warning: This endpoint is not meant to be machine parseable.

Started:  Wed Feb 18 22:09:26 UTC 2026
Up:  21 hr 43 min 15 sec
Go version:  go1.25.5
Binary version:  1.35.0
Emulation version:  1.35
Paths:  /configz /debug /flagz /healthz /metrics
`

	result, isJSON := parseStatusz([]byte(verboseResponse))

	assert.False(t, isJSON, "should detect text format (not JSON)")
	assert.Equal(t, "healthy", result.HealthStatus)

	// Text format should NOT create ComponentStatuses - only JSON format has real health data.
	assert.Empty(t, result.ComponentStatuses, "text format should not create synthetic component statuses")
}
