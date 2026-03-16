package metric

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

// Constants for repeated strings.
const (
	kubeletPrefix      = "kubelet"
	kubeletFlagsPrefix = "kubeletFlags"
	kubeletFlagPrefix  = "kubeletFlag"
	quantile99         = "0.99"
	statusHealthy      = "healthy"
	statusUnhealthy    = "unhealthy"
)

// Error variables for lint compliance.
var (
	ErrHTTPStatusError = errors.New("HTTP status error")
)

// flagsSplitParts is the number of parts expected when splitting a flag line on "=".
const flagsSplitParts = 2

// FlagsParser handles parsing kubelet flags from various formats.
type FlagsParser struct {
	logger *log.Logger
}

// NewFlagsParser creates a new FlagsParser.
func NewFlagsParser(logger *log.Logger) *FlagsParser {
	return &FlagsParser{
		logger: logger,
	}
}

// ParseIntoStruct extracts known flags from the map into the struct.
//
//nolint:funlen // Mapping many flags requires many statements.
func (p *FlagsParser) ParseIntoStruct(flagMap map[string]string, flags *KubeletFlags) {
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

// FlagsToRawMetrics converts KubeletFlags to RawMetrics.
//
//nolint:gocyclo,cyclop,funlen // Converting many flag fields requires many conditionals.
func (p *FlagsParser) FlagsToRawMetrics(flags *KubeletFlags) definition.RawMetrics {
	metrics := make(definition.RawMetrics)

	// Add a flags fingerprint for drift detection.
	fingerprint := p.CalculateFlagsFingerprint(flags)
	metrics["kubeletFlagsFingerprint"] = fingerprint

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

	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform).
	// This needs to be a map[string]interface{}, not a JSON string.
	// Strip component-specific prefixes to get clean names like "Fingerprint", "Address".
	flagsDiagnostics := make(map[string]interface{})
	for k, v := range metrics {
		key := stripKubeletPrefix(k)
		flagsDiagnostics[key] = v
	}
	metrics["kubeletFlagsDiagnostics"] = flagsDiagnostics

	return metrics
}

// CalculateFlagsFingerprint generates a SHA256 hash of the flags for drift detection.
func (p *FlagsParser) CalculateFlagsFingerprint(flags *KubeletFlags) string {
	// Create a normalized string representation of key flags.
	// We'll include flags that are most likely to differ across nodes.
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

	return fingerprint
}

// stripKubeletPrefix removes kubelet-related prefixes from metric keys.
func stripKubeletPrefix(k string) string {
	switch {
	case len(k) > 12 && k[:12] == kubeletFlagsPrefix:
		return k[12:] // kubeletFlagsFingerprint -> Fingerprint
	case len(k) > 11 && k[:11] == kubeletFlagPrefix:
		return k[11:] // kubeletFlagAddress -> Address
	case len(k) > 7 && k[:7] == kubeletPrefix:
		return k[7:] // fallback
	default:
		return k
	}
}

// parseBool parses a string boolean with a default value.
func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return defaultVal
	}
	return v
}
