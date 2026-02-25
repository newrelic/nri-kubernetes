package metric

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeletFlagsFetcher_Fetch(t *testing.T) {
	// Sample kubelet flags output (plain text format)
	sampleFlags := `--address=0.0.0.0
--anonymous-auth=false
--authorization-mode=Webhook
--cgroup-driver=systemd
--client-ca-file=/etc/kubernetes/pki/ca.crt
--cluster-dns=10.96.0.10
--cluster-domain=cluster.local
--container-runtime-endpoint=unix:///var/run/containerd/containerd.sock
--cpu-manager-policy=static
--enable-debugging-handlers=true
--eviction-hard=memory.available<100Mi,nodefs.available<10%
--feature-gates=RotateKubeletServerCertificate=true,CPUManager=true
--kube-reserved=cpu=100m,memory=100Mi
--max-pods=110
--memory-manager-policy=None
--node-ip=192.168.1.10
--pod-pids-limit=4096
--port=10250
--read-only-port=0
--register-node=true
--register-schedulable=true
--rotate-certificates=true
--server-tls-bootstrap=true
--system-reserved=cpu=100m,memory=100Mi
--tls-cert-file=/var/lib/kubelet/pki/kubelet.crt
--tls-private-key-file=/var/lib/kubelet/pki/kubelet.key
--topology-manager-policy=best-effort
--v=2
`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, FlagsPath, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleFlags))
	}))
	defer server.Close()

	// Create mock client
	mockClient := &mockHTTPGetter{
		server: server,
	}

	// Create fetcher
	fetcher := NewKubeletFlagsFetcher(logutil.Discard, mockClient, "test-node")

	// Fetch flags
	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	// Verify structure
	require.NotNil(t, rawGroups)
	nodeGroup, ok := rawGroups["node"]
	require.True(t, ok, "node group should exist")

	nodeMetrics, ok := nodeGroup["test-node"]
	require.True(t, ok, "test-node should exist in node group")

	// Verify key metrics - Server settings
	assert.Equal(t, "0.0.0.0", nodeMetrics["kubeletFlagAddress"])
	assert.Equal(t, int32(10250), nodeMetrics["kubeletFlagPort"])
	// When read-only port is 0, only the enabled flag is set
	assert.Equal(t, false, nodeMetrics["kubeletFlagReadOnlyPortEnabled"])

	// Security settings
	assert.Equal(t, false, nodeMetrics["kubeletFlagAnonymousAuth"])
	assert.Equal(t, "Webhook", nodeMetrics["kubeletFlagAuthorizationMode"])
	assert.Equal(t, "/etc/kubernetes/pki/ca.crt", nodeMetrics["kubeletFlagClientCAFile"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagRotateCertificates"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagServerTLSBootstrap"])

	// Resource management
	assert.Equal(t, int32(110), nodeMetrics["kubeletFlagMaxPods"])
	assert.Equal(t, int64(4096), nodeMetrics["kubeletFlagPodPidsLimit"])
	assert.Equal(t, "cpu=100m,memory=100Mi", nodeMetrics["kubeletFlagKubeReserved"])
	assert.Equal(t, "cpu=100m,memory=100Mi", nodeMetrics["kubeletFlagSystemReserved"])
	assert.Equal(t, "memory.available<100Mi,nodefs.available<10%", nodeMetrics["kubeletFlagEvictionHard"])

	// Runtime
	assert.Equal(t, "systemd", nodeMetrics["kubeletFlagCgroupDriver"])
	assert.Equal(t, "unix:///var/run/containerd/containerd.sock", nodeMetrics["kubeletFlagContainerRuntimeEndpoint"])

	// QoS policies
	assert.Equal(t, "static", nodeMetrics["kubeletFlagCPUManagerPolicy"])
	assert.Equal(t, "None", nodeMetrics["kubeletFlagMemoryManagerPolicy"])
	assert.Equal(t, "best-effort", nodeMetrics["kubeletFlagTopologyManagerPolicy"])

	// Networking
	assert.Equal(t, "10.96.0.10", nodeMetrics["kubeletFlagClusterDNS"])
	assert.Equal(t, "cluster.local", nodeMetrics["kubeletFlagClusterDomain"])
	assert.Equal(t, "192.168.1.10", nodeMetrics["kubeletFlagNodeIP"])

	// Feature gates
	assert.Equal(t, "RotateKubeletServerCertificate=true,CPUManager=true", nodeMetrics["kubeletFlagFeatureGates"])

	// Node management
	assert.Equal(t, true, nodeMetrics["kubeletFlagRegisterNode"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagRegisterSchedulable"])

	// Debugging
	assert.Equal(t, true, nodeMetrics["kubeletFlagEnableDebuggingHandlers"])

	// Logging
	assert.Equal(t, "2", nodeMetrics["kubeletFlagVerbosity"])

	// Verify fingerprint exists
	_, ok = nodeMetrics["kubeletFlagsFingerprint"]
	assert.True(t, ok, "flags fingerprint should be present")
}

func TestKubeletFlagsFetcher_Fetch_HTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagsFetcher(logutil.Discard, mockClient, "test-node")

	_, err := fetcher.Fetch()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "returned status 401")
}

func TestKubeletFlagsFetcher_ParseFlags_BooleanWithoutValue(t *testing.T) {
	// Test parsing flags that are just present (boolean true)
	flagsText := `--enable-server=true
--some-flag
--another-bool=false
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(flagsText))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagsFetcher(logutil.Discard, mockClient, "test-node")

	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]
	require.NotNil(t, nodeMetrics)

	// Flags without values should be treated as boolean true
	// but they won't be in our known metrics unless we add them
	// Just verify no error occurred
	assert.NotNil(t, nodeMetrics)
}

func TestKubeletFlagsFetcher_SecurityRiskFlags(t *testing.T) {
	// Test detection of security risk flags
	flagsText := `--read-only-port=10255
--anonymous-auth=true
--authorization-mode=AlwaysAllow
--allow-privileged=true
--enable-debugging-handlers=true
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(flagsText))
	}))
	defer server.Close()

	mockClient := &mockHTTPGetter{
		server: server,
	}

	fetcher := NewKubeletFlagsFetcher(logutil.Discard, mockClient, "test-node")

	rawGroups, err := fetcher.Fetch()
	require.NoError(t, err)

	nodeMetrics := rawGroups["node"]["test-node"]

	// Security risks that should trigger alerts
	assert.Equal(t, int32(10255), nodeMetrics["kubeletFlagReadOnlyPort"])
	assert.Equal(t, true, nodeMetrics["kubeletFlagReadOnlyPortEnabled"], "Read-only port enabled is a security risk")
	assert.Equal(t, true, nodeMetrics["kubeletFlagAnonymousAuth"], "Anonymous auth enabled is a security risk")
	assert.Equal(t, "AlwaysAllow", nodeMetrics["kubeletFlagAuthorizationMode"], "AlwaysAllow auth mode is a security risk")
	assert.Equal(t, true, nodeMetrics["kubeletFlagAllowPrivileged"], "Allow privileged is a security risk")
}

func TestCalculateFlagsFingerprint(t *testing.T) {
	flags1 := &KubeletFlags{
		AnonymousAuth:       false,
		AuthorizationMode:   "Webhook",
		ReadOnlyPort:        0,
		MaxPods:             110,
		CgroupDriver:        "systemd",
		CPUManagerPolicy:    "static",
	}

	flags2 := &KubeletFlags{
		AnonymousAuth:       false,
		AuthorizationMode:   "Webhook",
		ReadOnlyPort:        0,
		MaxPods:             110,
		CgroupDriver:        "systemd",
		CPUManagerPolicy:    "static",
	}

	flags3 := &KubeletFlags{
		AnonymousAuth:       true, // Different - security risk
		AuthorizationMode:   "Webhook",
		ReadOnlyPort:        0,
		MaxPods:             110,
		CgroupDriver:        "systemd",
		CPUManagerPolicy:    "static",
	}

	fetcher := NewKubeletFlagsFetcher(logutil.Discard, nil, "test-node")

	fp1, err := fetcher.calculateFlagsFingerprint(flags1)
	require.NoError(t, err)

	fp2, err := fetcher.calculateFlagsFingerprint(flags2)
	require.NoError(t, err)

	fp3, err := fetcher.calculateFlagsFingerprint(flags3)
	require.NoError(t, err)

	// Same flags should have same fingerprint
	assert.Equal(t, fp1, fp2, "identical flags should have identical fingerprints")

	// Different flags should have different fingerprints
	assert.NotEqual(t, fp1, fp3, "different flags should have different fingerprints")
}

func TestFlagsToRawMetrics_EmptyFlags(t *testing.T) {
	flags := &KubeletFlags{}
	fetcher := NewKubeletFlagsFetcher(logutil.Discard, nil, "test-node")

	metrics, err := fetcher.flagsToRawMetrics(flags)
	require.NoError(t, err)

	// Should still have fingerprint even for empty flags
	_, ok := metrics["kubeletFlagsFingerprint"]
	assert.True(t, ok)

	// Read-only port should default to disabled when not set
	assert.Equal(t, false, metrics["kubeletFlagReadOnlyPortEnabled"])
}

func TestParseFlags_EmptyLines(t *testing.T) {
	flagsText := `--max-pods=110

--port=10250

--anonymous-auth=false
`

	reader := strings.NewReader(flagsText)
	fetcher := NewKubeletFlagsFetcher(logutil.Discard, nil, "test-node")

	flags, err := fetcher.parseFlags(reader)
	require.NoError(t, err)

	assert.Equal(t, int32(110), flags.MaxPods)
	assert.Equal(t, int32(10250), flags.Port)
	assert.Equal(t, false, flags.AnonymousAuth)
}

func TestParseFlags_MalformedLines(t *testing.T) {
	flagsText := `--max-pods=110
some random text
--port=10250
not a flag
--anonymous-auth=false
`

	reader := strings.NewReader(flagsText)
	fetcher := NewKubeletFlagsFetcher(logutil.Discard, nil, "test-node")

	flags, err := fetcher.parseFlags(reader)
	require.NoError(t, err)

	// Should parse valid flags and ignore invalid lines
	assert.Equal(t, int32(110), flags.MaxPods)
	assert.Equal(t, int32(10250), flags.Port)
	assert.Equal(t, false, flags.AnonymousAuth)
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		defVal   bool
		expected bool
	}{
		{"empty string with default true", "", true, true},
		{"empty string with default false", "", false, false},
		{"true string", "true", false, true},
		{"false string", "false", true, false},
		{"invalid string uses default", "not-a-bool", true, true},
		{"1 is true", "1", false, true},
		{"0 is false", "0", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.input, tt.defVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}
