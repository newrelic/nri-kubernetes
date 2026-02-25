package metric

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeletConfigFetcher_Fetch(t *testing.T) {
	// Sample kubelet config response (simplified for testing)
	sampleConfig := kubeletConfigResponse{
		ComponentConfig: KubeletConfigSnapshot{
			MaxPods:                     intPtr(110),
			PodPidsLimit:                int64Ptr(4096),
			ImageGCHighThresholdPercent: intPtr(85),
			ImageGCLowThresholdPercent:  intPtr(80),
			EvictionHard: map[string]string{
				"memory.available":  "100Mi",
				"nodefs.available":  "10%",
				"imagefs.available": "15%",
			},
			CPUManagerPolicy:      strPtr("static"),
			MemoryManagerPolicy:   strPtr("None"),
			TopologyManagerPolicy: strPtr("best-effort"),
			ProtectKernelDefaults: boolPtr(true),
			SeccompDefault:        boolPtr(true),
			Authentication: &KubeletAuthentication{
				Anonymous: &KubeletAnonymous{
					Enabled: boolPtr(false),
				},
				Webhook: &KubeletWebhook{
					Enabled: boolPtr(true),
				},
			},
			Authorization: &KubeletAuthorization{
				Mode: strPtr("Webhook"),
			},
			FeatureGates: map[string]bool{
				"CPUManager":    true,
				"RotateKubeletServerCertificate": true,
				"SomeFeature":   false,
			},
			ClusterDNS:    []string{"10.96.0.10"},
			ClusterDomain: strPtr("cluster.local"),
			CgroupDriver:  strPtr("systemd"),
			Port:          intPtr(10250),
			ReadOnlyPort:  intPtr(0),
		},
	}

	configJSON, err := json.Marshal(sampleConfig)
	require.NoError(t, err)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, ConfigzPath, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(configJSON)
	}))
	defer server.Close()

	// Create mock client
	mockClient := &mockHTTPGetter{
		server: server,
	}

	// Create fetcher
	fetcher := NewKubeletConfigFetcher(logutil.Discard, mockClient, "test-node")

	// Fetch config
	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	// Verify structure
	require.NotNil(t, rawGroups)
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok, "node group should exist")

	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok, "test-node should exist in node group")

	// Verify key metrics
	assert.Equal(t, int32(110), nodeMetrics["kubeletMaxPods"])
	assert.Equal(t, int64(4096), nodeMetrics["kubeletPodPidsLimit"])
	assert.Equal(t, "static", nodeMetrics["kubeletCPUManagerPolicy"])
	assert.Equal(t, "systemd", nodeMetrics["kubeletCgroupDriver"])
	assert.Equal(t, false, nodeMetrics["kubeletAnonymousAuthEnabled"])
	assert.Equal(t, "Webhook", nodeMetrics["kubeletAuthorizationMode"])
	assert.Equal(t, true, nodeMetrics["kubeletProtectKernelDefaults"])
	assert.Equal(t, int32(0), nodeMetrics["kubeletReadOnlyPort"])
	assert.Equal(t, false, nodeMetrics["kubeletReadOnlyPortEnabled"])

	// Verify feature gates count
	assert.Equal(t, 2, nodeMetrics["kubeletFeatureGatesEnabledCount"])

	// Verify JSON fields
	evictionHardJSON, ok := nodeMetrics["kubeletEvictionHard"].(string)
	require.True(t, ok, "kubeletEvictionHard should be a JSON string")
	var evictionHard map[string]string
	err = json.Unmarshal([]byte(evictionHardJSON), &evictionHard)
	require.NoError(t, err)
	assert.Equal(t, "100Mi", evictionHard["memory.available"])

	// Verify fingerprint exists
	_, ok = nodeMetrics["kubeletConfigFingerprint"]
	assert.True(t, ok, "config fingerprint should be present")
}

func TestKubeletConfigFetcher_Fetch_HTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletConfigFetcher(logutil.Discard, mockClient, "test-node")

	_, err := fetcher.Fetch()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returned status 403")
}

func TestKubeletConfigFetcher_Fetch_InvalidJSON(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletConfigFetcher(logutil.Discard, mockClient, "test-node")

	_, err := fetcher.Fetch()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error unmarshaling")
}

func TestCalculateConfigFingerprint(t *testing.T) {
	config1 := &KubeletConfigSnapshot{
		MaxPods:           intPtr(110),
		CPUManagerPolicy:  strPtr("static"),
	}

	config2 := &KubeletConfigSnapshot{
		MaxPods:           intPtr(110),
		CPUManagerPolicy:  strPtr("static"),
	}

	config3 := &KubeletConfigSnapshot{
		MaxPods:           intPtr(100), // Different value
		CPUManagerPolicy:  strPtr("static"),
	}

	fetcher := NewKubeletConfigFetcher(logutil.Discard, nil, "test-node")

	fp1, err := fetcher.calculateConfigFingerprint(config1)
	require.NoError(t, err)

	fp2, err := fetcher.calculateConfigFingerprint(config2)
	require.NoError(t, err)

	fp3, err := fetcher.calculateConfigFingerprint(config3)
	require.NoError(t, err)

	// Same configs should have same fingerprint
	assert.Equal(t, fp1, fp2)

	// Different configs should have different fingerprints
	assert.NotEqual(t, fp1, fp3)
}

func TestConfigToRawMetrics_EmptyConfig(t *testing.T) {
	config := &KubeletConfigSnapshot{}
	fetcher := NewKubeletConfigFetcher(logutil.Discard, nil, "test-node")

	metrics, err := fetcher.configToRawMetrics(config)
	require.NoError(t, err)

	// Should still have fingerprint even for empty config
	_, ok := metrics["kubeletConfigFingerprint"]
	assert.True(t, ok)
}

// Mock HTTP client for testing
type mockHTTPGetter struct {
	server *httptest.Server
}

func (m *mockHTTPGetter) Get(urlPath string) (*http.Response, error) {
	// Build full URL
	fullURL := fmt.Sprintf("%s%s", m.server.URL, urlPath)
	return http.Get(fullURL)
}

func (m *mockHTTPGetter) GetURI(uri url.URL) (*http.Response, error) {
	return http.Get(uri.String())
}

func (m *mockHTTPGetter) Do(req *http.Request) (*http.Response, error) {
	// Build full URL by replacing the request URL with the test server URL
	fullURL := fmt.Sprintf("%s%s", m.server.URL, req.URL.Path)
	req.URL, _ = url.Parse(fullURL)
	return http.DefaultClient.Do(req)
}

// Helper functions for pointer creation
func intPtr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
