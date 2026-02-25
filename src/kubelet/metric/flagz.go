package metric

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	// FlagzPath is the path where kubelet exposes its command-line flags
	// Requires ComponentFlagz feature gate (alpha in 1.32+)
	FlagzPath = "/flagz"
)

// KubeletFlagzFetcher queries the kubelet /flagz endpoint to fetch command-line flags.
// The /flagz endpoint requires the ComponentFlagz feature gate to be enabled.
// It returns plain text key=value format (same as the legacy /flags endpoint).
type KubeletFlagzFetcher struct {
	logger   *log.Logger
	client   client.HTTPGetter
	nodeName string
}

// NewKubeletFlagzFetcher creates a new KubeletFlagzFetcher
func NewKubeletFlagzFetcher(logger *log.Logger, client client.HTTPGetter, nodeName string) *KubeletFlagzFetcher {
	return &KubeletFlagzFetcher{
		logger:   logger,
		client:   client,
		nodeName: nodeName,
	}
}

// Fetch retrieves the kubelet flags from the /flagz endpoint and returns them as RawGroups.
// The /flagz endpoint returns plain text key=value format.
func (f *KubeletFlagzFetcher) Fetch() (definition.RawGroups, error) {
	f.logger.Debugf("Fetching kubelet flags from %s", FlagzPath)

	var resp *http.Response
	var err error

	// Use content negotiation to request text/plain format (flagz always returns text)
	if clientWithAccept, ok := f.client.(client.HTTPGetterWithAccept); ok {
		resp, err = clientWithAccept.GetWithAccept(FlagzPath, "text/plain")
	} else {
		resp, err = f.client.Get(FlagzPath)
	}
	if err != nil {
		return nil, fmt.Errorf("error calling kubelet %s path: %w", FlagzPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kubelet %s returned status %d: %s", FlagzPath, resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from kubelet %s: %w", FlagzPath, err)
	}

	// Verify Content-Type (flagz always returns text/plain)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "text/plain") {
		f.logger.Debugf("Unexpected Content-Type from %s: %s (expected text/plain)", FlagzPath, contentType)
	}

	// Parse plain text key=value format (same format as /flags)
	flagMap := f.parsePlainTextFlags(string(body))

	// Convert to KubeletFlags structure
	flags := &KubeletFlags{}
	f.parseIntoStruct(flagMap, flags)

	// Convert flags to RawMetrics
	rawMetrics, err := f.flagsToRawMetrics(flags)
	if err != nil {
		return nil, fmt.Errorf("error converting flags to raw metrics: %w", err)
	}

	rawGroups := definition.RawGroups{
		"node": {
			f.nodeName: rawMetrics,
		},
	}

	return rawGroups, nil
}

// parsePlainTextFlags parses the plain text key=value format from /flagz
// Format: "flag=value" lines (without leading --)
func (f *KubeletFlagzFetcher) parsePlainTextFlags(body string) map[string]string {
	flagMap := make(map[string]string)

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines, headers, and warnings
		if line == "" || strings.HasPrefix(line, "kubelet") || strings.HasPrefix(line, "Warning:") {
			continue
		}

		// Split on first =
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		flagName := strings.TrimSpace(parts[0])
		flagValue := strings.TrimSpace(parts[1])
		flagMap[flagName] = flagValue
	}

	return flagMap
}

// parseIntoStruct extracts known flags from the map into the struct.
// This is identical to the flags.go implementation to ensure consistent output.
func (f *KubeletFlagzFetcher) parseIntoStruct(flagMap map[string]string, flags *KubeletFlags) {
	// Server Settings
	flags.Address = flagMap["address"]
	if v, err := strconv.ParseInt(flagMap["port"], 10, 32); err == nil {
		flags.Port = int32(v)
	}
	if v, err := strconv.ParseInt(flagMap["read-only-port"], 10, 32); err == nil {
		flags.ReadOnlyPort = int32(v)
	}

	// Security Settings
	flags.AnonymousAuth = parseBool(flagMap["anonymous-auth"], false)
	flags.AuthorizationMode = flagMap["authorization-mode"]
	flags.ClientCAFile = flagMap["client-ca-file"]
	flags.TLSCertFile = flagMap["tls-cert-file"]
	flags.TLSPrivateKeyFile = flagMap["tls-private-key-file"]
	flags.RotateCertificates = parseBool(flagMap["rotate-certificates"], false)
	flags.ServerTLSBootstrap = parseBool(flagMap["server-tls-bootstrap"], false)

	// Resource Management
	if v, err := strconv.ParseInt(flagMap["max-pods"], 10, 32); err == nil {
		flags.MaxPods = int32(v)
	}
	if v, err := strconv.ParseInt(flagMap["pod-pids-limit"], 10, 64); err == nil {
		flags.PodPidsLimit = v
	}
	flags.KubeReserved = flagMap["kube-reserved"]
	flags.SystemReserved = flagMap["system-reserved"]
	flags.EvictionHard = flagMap["eviction-hard"]
	flags.EvictionSoft = flagMap["eviction-soft"]

	// Container Runtime
	flags.ContainerRuntime = flagMap["container-runtime"]
	flags.ContainerRuntimeEndpoint = flagMap["container-runtime-endpoint"]
	flags.ImageServiceEndpoint = flagMap["image-service-endpoint"]
	flags.CgroupDriver = flagMap["cgroup-driver"]
	flags.CgroupRoot = flagMap["cgroup-root"]
	flags.RuntimeRequestTimeout = flagMap["runtime-request-timeout"]

	// Pod Management
	flags.PodManifestPath = flagMap["pod-manifest-path"]
	flags.ManifestURL = flagMap["manifest-url"]
	flags.ManifestURLHeader = flagMap["manifest-url-header"]
	flags.SyncFrequency = flagMap["sync-frequency"]
	flags.FileCheckFrequency = flagMap["file-check-frequency"]
	flags.HTTPCheckFrequency = flagMap["http-check-frequency"]

	// Networking
	flags.ClusterDNS = flagMap["cluster-dns"]
	flags.ClusterDomain = flagMap["cluster-domain"]
	flags.NetworkPlugin = flagMap["network-plugin"]
	flags.CNIBinDir = flagMap["cni-bin-dir"]
	flags.CNIConfDir = flagMap["cni-conf-dir"]
	flags.PodCIDR = flagMap["pod-cidr"]
	flags.NodeIP = flagMap["node-ip"]

	// Feature Gates
	flags.FeatureGates = flagMap["feature-gates"]

	// Node Management
	flags.RegisterNode = parseBool(flagMap["register-node"], true)
	flags.RegisterSchedulable = parseBool(flagMap["register-schedulable"], true)
	flags.NodeLabels = flagMap["node-labels"]
	flags.NodeStatusUpdateFrequency = flagMap["node-status-update-frequency"]
	flags.NodeStatusReportFrequency = flagMap["node-status-report-frequency"]

	// Housekeeping
	flags.HousekeepingInterval = flagMap["housekeeping-interval"]
	if v, err := strconv.ParseInt(flagMap["image-gc-high-threshold"], 10, 32); err == nil {
		flags.ImageGCHighThresholdPercent = int32(v)
	}
	if v, err := strconv.ParseInt(flagMap["image-gc-low-threshold"], 10, 32); err == nil {
		flags.ImageGCLowThresholdPercent = int32(v)
	}
	flags.ImageMinimumGCAge = flagMap["image-minimum-gc-age"]

	// CPU/Memory Management
	flags.CPUManagerPolicy = flagMap["cpu-manager-policy"]
	flags.CPUManagerReconcilePeriod = flagMap["cpu-manager-reconcile-period"]
	flags.MemoryManagerPolicy = flagMap["memory-manager-policy"]
	flags.TopologyManagerPolicy = flagMap["topology-manager-policy"]
	flags.TopologyManagerScope = flagMap["topology-manager-scope"]
	flags.ReservedSystemCPUs = flagMap["reserved-system-cpus"]

	// Logging & Monitoring
	flags.LogLevel = flagMap["log-level"]
	flags.V = flagMap["v"]
	flags.VModule = flagMap["vmodule"]

	// Cloud Provider
	flags.CloudProvider = flagMap["cloud-provider"]
	flags.CloudConfig = flagMap["cloud-config"]

	// Deprecated/Security Risk Flags
	flags.EnableDebuggingHandlers = parseBool(flagMap["enable-debugging-handlers"], true)
	flags.EnableContentionProfiling = parseBool(flagMap["enable-contention-profiling"], false)
	flags.AllowPrivileged = parseBool(flagMap["allow-privileged"], false)
	flags.HostnameOverride = flagMap["hostname-override"]
}

// flagsToRawMetrics converts KubeletFlags to RawMetrics.
// This is identical to the flags.go implementation to ensure consistent output.
func (f *KubeletFlagzFetcher) flagsToRawMetrics(flags *KubeletFlags) (definition.RawMetrics, error) {
	metrics := make(definition.RawMetrics)

	// Add a flags fingerprint for drift detection
	fingerprint, err := f.calculateFlagsFingerprint(flags)
	if err != nil {
		f.logger.Warnf("Failed to calculate flags fingerprint: %v", err)
	} else {
		metrics["kubeletFlagsFingerprint"] = fingerprint
	}

	// Server Settings
	if flags.Address != "" {
		metrics["kubeletFlagAddress"] = flags.Address
	}
	if flags.Port != 0 {
		metrics["kubeletFlagPort"] = flags.Port
	}
	if flags.ReadOnlyPort != 0 {
		metrics["kubeletFlagReadOnlyPort"] = flags.ReadOnlyPort
		metrics["kubeletFlagReadOnlyPortEnabled"] = flags.ReadOnlyPort != 0
	} else {
		metrics["kubeletFlagReadOnlyPortEnabled"] = false
	}

	// Security Settings (critical for security auditing)
	metrics["kubeletFlagAnonymousAuth"] = flags.AnonymousAuth
	if flags.AuthorizationMode != "" {
		metrics["kubeletFlagAuthorizationMode"] = flags.AuthorizationMode
	}
	if flags.ClientCAFile != "" {
		metrics["kubeletFlagClientCAFile"] = flags.ClientCAFile
	}
	metrics["kubeletFlagRotateCertificates"] = flags.RotateCertificates
	metrics["kubeletFlagServerTLSBootstrap"] = flags.ServerTLSBootstrap

	// Resource Management
	if flags.MaxPods != 0 {
		metrics["kubeletFlagMaxPods"] = flags.MaxPods
	}
	if flags.PodPidsLimit != 0 {
		metrics["kubeletFlagPodPidsLimit"] = flags.PodPidsLimit
	}
	if flags.KubeReserved != "" {
		metrics["kubeletFlagKubeReserved"] = flags.KubeReserved
	}
	if flags.SystemReserved != "" {
		metrics["kubeletFlagSystemReserved"] = flags.SystemReserved
	}
	if flags.EvictionHard != "" {
		metrics["kubeletFlagEvictionHard"] = flags.EvictionHard
	}

	// Container Runtime
	if flags.ContainerRuntimeEndpoint != "" {
		metrics["kubeletFlagContainerRuntimeEndpoint"] = flags.ContainerRuntimeEndpoint
	}
	if flags.CgroupDriver != "" {
		metrics["kubeletFlagCgroupDriver"] = flags.CgroupDriver
	}

	// Networking
	if flags.ClusterDNS != "" {
		metrics["kubeletFlagClusterDNS"] = flags.ClusterDNS
	}
	if flags.ClusterDomain != "" {
		metrics["kubeletFlagClusterDomain"] = flags.ClusterDomain
	}
	if flags.NetworkPlugin != "" {
		metrics["kubeletFlagNetworkPlugin"] = flags.NetworkPlugin
	}
	if flags.NodeIP != "" {
		metrics["kubeletFlagNodeIP"] = flags.NodeIP
	}

	// Feature Gates
	if flags.FeatureGates != "" {
		metrics["kubeletFlagFeatureGates"] = flags.FeatureGates
	}

	// Node Management
	metrics["kubeletFlagRegisterNode"] = flags.RegisterNode
	metrics["kubeletFlagRegisterSchedulable"] = flags.RegisterSchedulable
	if flags.NodeLabels != "" {
		metrics["kubeletFlagNodeLabels"] = flags.NodeLabels
	}

	// CPU/Memory Management
	if flags.CPUManagerPolicy != "" {
		metrics["kubeletFlagCPUManagerPolicy"] = flags.CPUManagerPolicy
	}
	if flags.MemoryManagerPolicy != "" {
		metrics["kubeletFlagMemoryManagerPolicy"] = flags.MemoryManagerPolicy
	}
	if flags.TopologyManagerPolicy != "" {
		metrics["kubeletFlagTopologyManagerPolicy"] = flags.TopologyManagerPolicy
	}
	if flags.ReservedSystemCPUs != "" {
		metrics["kubeletFlagReservedSystemCPUs"] = flags.ReservedSystemCPUs
	}

	// Cloud Provider
	if flags.CloudProvider != "" {
		metrics["kubeletFlagCloudProvider"] = flags.CloudProvider
	}

	// Security Risk Flags (for alerting)
	metrics["kubeletFlagEnableDebuggingHandlers"] = flags.EnableDebuggingHandlers
	metrics["kubeletFlagEnableContentionProfiling"] = flags.EnableContentionProfiling
	metrics["kubeletFlagAllowPrivileged"] = flags.AllowPrivileged
	if flags.HostnameOverride != "" {
		metrics["kubeletFlagHostnameOverride"] = flags.HostnameOverride
	}

	// Log level
	if flags.LogLevel != "" {
		metrics["kubeletFlagLogLevel"] = flags.LogLevel
	}
	if flags.V != "" {
		metrics["kubeletFlagVerbosity"] = flags.V
	}

	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform)
	// This needs to be a map[string]interface{}, not a JSON string
	// Strip component-specific prefixes to get clean names like "Fingerprint", "Address"
	flagsDiagnostics := make(map[string]interface{})
	for k, v := range metrics {
		var key string
		switch {
		case len(k) > 12 && k[:12] == "kubeletFlags":
			key = k[12:] // kubeletFlagsFingerprint -> Fingerprint
		case len(k) > 11 && k[:11] == "kubeletFlag":
			key = k[11:] // kubeletFlagAddress -> Address
		case len(k) > 7 && k[:7] == "kubelet":
			key = k[7:] // fallback
		default:
			key = k
		}
		flagsDiagnostics[key] = v
	}
	metrics["kubeletFlagsDiagnostics"] = flagsDiagnostics

	return metrics, nil
}

// calculateFlagsFingerprint generates a SHA256 hash of the flags for drift detection.
// This is identical to the flags.go implementation to ensure consistent fingerprints.
func (f *KubeletFlagzFetcher) calculateFlagsFingerprint(flags *KubeletFlags) (string, error) {
	// Create a normalized string representation of key flags
	// We'll include flags that are most likely to differ across nodes
	var sb strings.Builder

	// Security-critical flags
	fmt.Fprintf(&sb, "anonymous-auth=%v|", flags.AnonymousAuth)
	fmt.Fprintf(&sb, "authorization-mode=%s|", flags.AuthorizationMode)
	fmt.Fprintf(&sb, "read-only-port=%d|", flags.ReadOnlyPort)

	// Resource management
	fmt.Fprintf(&sb, "max-pods=%d|", flags.MaxPods)
	fmt.Fprintf(&sb, "pod-pids-limit=%d|", flags.PodPidsLimit)
	fmt.Fprintf(&sb, "kube-reserved=%s|", flags.KubeReserved)
	fmt.Fprintf(&sb, "system-reserved=%s|", flags.SystemReserved)
	fmt.Fprintf(&sb, "eviction-hard=%s|", flags.EvictionHard)

	// Runtime
	fmt.Fprintf(&sb, "cgroup-driver=%s|", flags.CgroupDriver)
	fmt.Fprintf(&sb, "container-runtime-endpoint=%s|", flags.ContainerRuntimeEndpoint)

	// QoS policies
	fmt.Fprintf(&sb, "cpu-manager-policy=%s|", flags.CPUManagerPolicy)
	fmt.Fprintf(&sb, "memory-manager-policy=%s|", flags.MemoryManagerPolicy)
	fmt.Fprintf(&sb, "topology-manager-policy=%s|", flags.TopologyManagerPolicy)

	// Feature gates
	fmt.Fprintf(&sb, "feature-gates=%s", flags.FeatureGates)

	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(sb.String()))
	fingerprint := fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes (16 hex chars) for brevity

	return fingerprint, nil
}
