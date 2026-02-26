package metric

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample kubelet /flagz response (plain text format as returned by Kubernetes 1.32+).
const sampleFlagzResponse = `kubelet flagz
Warning: This endpoint is not meant to be machine parseable, has no formatting compatibility guarantees and is for debugging purposes only.
address=0.0.0.0
anonymous-auth=false
authorization-mode=Webhook
cgroup-driver=systemd
client-ca-file=/etc/kubernetes/pki/ca.crt
cluster-dns=10.96.0.10
cluster-domain=cluster.local
container-runtime-endpoint=unix:///var/run/containerd/containerd.sock
cpu-manager-policy=static
enable-debugging-handlers=true
eviction-hard=memory.available<100Mi,nodefs.available<10%
feature-gates=RotateKubeletServerCertificate=true,CPUManager=true
kube-reserved=cpu=100m,memory=100Mi
max-pods=110
memory-manager-policy=None
node-ip=192.168.1.10
pod-pids-limit=4096
port=10250
read-only-port=0
register-node=true
register-schedulable=true
rotate-certificates=true
server-tls-bootstrap=true
system-reserved=cpu=100m,memory=100Mi
topology-manager-policy=best-effort
v=2
`

func TestKubeletFlagzFetcher_Fetch(t *testing.T) {
	t.Parallel()
	// Create test server that returns plain text format.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, FlagzPath, r.URL.Path)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFlagzResponse))
	}))
	defer server.Close()

	// Create mock client.
	mockClient := &mockHTTPGetter{
		server: server,
	}

	// Create fetcher.
	fetcher := NewKubeletFlagzFetcher(logutil.Discard, mockClient, "test-node")

	// Fetch flags.
	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	// Verify structure.
	require.NotNil(t, rawGroups)
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok, "node group should exist")

	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok, "test-node should exist in node group")

	// Verify key metrics - Server settings.
	assert.Equal(t, "0.0.0.0", nodeMetrics["kubeletFlagAddress"])
	assert.Equal(t, int32(10250), nodeMetrics["kubeletFlagPort"])
	// When read-only port is 0, only the enabled flag is set.
	assert.Equal(t, false, nodeMetrics["kubeletFlagReadOnlyPortEnabled"])

	// Security settings.
	assert.Equal(t, false, nodeMetrics["kubeletFlagAnonymousAuth"])
	assert.Equal(t, "Webhook", nodeMetrics["kubeletFlagAuthorizationMode"])
	assert.Equal(t, "/etc/kubernetes/pki/ca.crt", nodeMetrics["kubeletFlagClientCAFile"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagRotateCertificates"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagServerTLSBootstrap"])

	// Resource management.
	assert.Equal(t, int32(110), nodeMetrics["kubeletFlagMaxPods"])
	assert.Equal(t, int64(4096), nodeMetrics["kubeletFlagPodPidsLimit"])
	assert.Equal(t, "cpu=100m,memory=100Mi", nodeMetrics["kubeletFlagKubeReserved"])
	assert.Equal(t, "cpu=100m,memory=100Mi", nodeMetrics["kubeletFlagSystemReserved"])
	assert.Equal(t, "memory.available<100Mi,nodefs.available<10%", nodeMetrics["kubeletFlagEvictionHard"])

	// Runtime.
	assert.Equal(t, "systemd", nodeMetrics["kubeletFlagCgroupDriver"])
	assert.Equal(t, "unix:///var/run/containerd/containerd.sock", nodeMetrics["kubeletFlagContainerRuntimeEndpoint"])

	// QoS policies.
	assert.Equal(t, "static", nodeMetrics["kubeletFlagCPUManagerPolicy"])
	assert.Equal(t, "None", nodeMetrics["kubeletFlagMemoryManagerPolicy"])
	assert.Equal(t, "best-effort", nodeMetrics["kubeletFlagTopologyManagerPolicy"])

	// Networking.
	assert.Equal(t, "10.96.0.10", nodeMetrics["kubeletFlagClusterDNS"])
	assert.Equal(t, "cluster.local", nodeMetrics["kubeletFlagClusterDomain"])
	assert.Equal(t, "192.168.1.10", nodeMetrics["kubeletFlagNodeIP"])

	// Feature gates.
	assert.Equal(t, "RotateKubeletServerCertificate=true,CPUManager=true", nodeMetrics["kubeletFlagFeatureGates"])

	// Node management.
	assert.Equal(t, true, nodeMetrics["kubeletFlagRegisterNode"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagRegisterSchedulable"])

	// Debugging.
	assert.Equal(t, true, nodeMetrics["kubeletFlagEnableDebuggingHandlers"])

	// Logging.
	assert.Equal(t, "2", nodeMetrics["kubeletFlagVerbosity"])

	// Verify fingerprint exists.
	_, ok = nodeMetrics["kubeletFlagsFingerprint"]
	assert.True(t, ok, "flags fingerprint should be present")
}

//nolint:dupl // Test structure is similar to other HTTP error tests but tests different component.
func TestKubeletFlagzFetcher_Fetch_HTTPError(t *testing.T) {
	t.Parallel()
	// Create test server that returns error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagzFetcher(logutil.Discard, mockClient, "test-node")

	_, err := fetcher.Fetch()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returned status 401")
}

//nolint:dupl // Test structure is similar to other HTTP error tests but tests different component.
func TestKubeletFlagzFetcher_Fetch_NotFound(t *testing.T) {
	t.Parallel()
	// Test handling of 404 (endpoint doesn't exist without ComponentFlagz feature gate).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 page not found"))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagzFetcher(logutil.Discard, mockClient, "test-node")

	_, err := fetcher.Fetch()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returned status 404")
}

func TestKubeletFlagzFetcher_SecurityRiskFlags(t *testing.T) {
	t.Parallel()
	// Test detection of security risk flags.
	securityRiskResponse := `kubelet flagz
Warning: This endpoint is not meant to be machine parseable.
read-only-port=10255
anonymous-auth=true
authorization-mode=AlwaysAllow
allow-privileged=true
enable-debugging-handlers=true
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(securityRiskResponse))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagzFetcher(logutil.Discard, mockClient, "test-node")

	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Security risks that should trigger alerts.
	assert.Equal(t, int32(10255), nodeMetrics["kubeletFlagReadOnlyPort"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagReadOnlyPortEnabled"], "Read-only port enabled is a security risk")
	assert.Equal(t, true, nodeMetrics["kubeletFlagAnonymousAuth"], "Anonymous auth enabled is a security risk")
	assert.Equal(t, "AlwaysAllow", nodeMetrics["kubeletFlagAuthorizationMode"], "AlwaysAllow auth mode is a security risk")
	assert.Equal(t, true, nodeMetrics["kubeletFlagAllowPrivileged"], "Allow privileged is a security risk")
}

func TestKubeletFlagzFetcher_EmptyFlags(t *testing.T) {
	t.Parallel()
	// Test handling of empty/minimal response.
	emptyResponse := `kubelet flagz
Warning: This endpoint is not meant to be machine parseable.
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyResponse))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagzFetcher(logutil.Discard, mockClient, "test-node")

	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Should still have fingerprint even for empty flags.
	_, ok := nodeMetrics["kubeletFlagsFingerprint"]
	assert.True(t, ok)

	// Read-only port should default to disabled when not set.
	assert.Equal(t, false, nodeMetrics["kubeletFlagReadOnlyPortEnabled"])
}

func TestKubeletFlagzFetcher_FingerprintConsistency(t *testing.T) {
	t.Parallel()
	// Verify that the shared parser produces consistent fingerprints.
	flags1 := &KubeletFlags{
		AnonymousAuth:     false,
		AuthorizationMode: "Webhook",
		ReadOnlyPort:      0,
		MaxPods:           110,
		CgroupDriver:      "systemd",
		CPUManagerPolicy:  "static",
	}

	flags2 := &KubeletFlags{
		AnonymousAuth:     false,
		AuthorizationMode: "Webhook",
		ReadOnlyPort:      0,
		MaxPods:           110,
		CgroupDriver:      "systemd",
		CPUManagerPolicy:  "static",
	}

	// Both flagz and flags fetchers use the same shared parser.
	parser := NewFlagsParser(logutil.Discard)

	fp1 := parser.CalculateFlagsFingerprint(flags1)
	fp2 := parser.CalculateFlagsFingerprint(flags2)

	// Same flags should produce the same fingerprint.
	assert.Equal(t, fp1, fp2, "same flags should produce identical fingerprints")
}

func TestKubeletFlagzFetcher_ParsePlainTextFlags(t *testing.T) {
	t.Parallel()
	fetcher := NewKubeletFlagzFetcher(logutil.Discard, nil, "test-node")

	testCases := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "standard format",
			input: `kubelet flagz
Warning: This endpoint is not meant to be machine parseable.
address=0.0.0.0
port=10250
anonymous-auth=false`,
			expected: map[string]string{
				"address":        "0.0.0.0",
				"port":           "10250",
				"anonymous-auth": "false",
			},
		},
		{
			name: "values with equals sign",
			input: `kubelet flagz
eviction-hard=memory.available<100Mi,nodefs.available<10%
feature-gates=CPUManager=true,RotateKubeletCert=true`,
			expected: map[string]string{
				"eviction-hard": "memory.available<100Mi,nodefs.available<10%",
				"feature-gates": "CPUManager=true,RotateKubeletCert=true",
			},
		},
		{
			name:     "empty response",
			input:    "",
			expected: map[string]string{},
		},
		{
			name: "only headers",
			input: `kubelet flagz
Warning: This endpoint is not meant to be machine parseable.`,
			expected: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := fetcher.parsePlainTextFlags(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
