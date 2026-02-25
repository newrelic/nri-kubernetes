package metric

import (
	"bufio"
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
	// FlagsPath is the path where kubelet exposes its command-line flags
	FlagsPath = "/flags"
)

// KubeletFlagsFetcher queries the kubelet /flags endpoint to fetch command-line flags.
type KubeletFlagsFetcher struct {
	logger   *log.Logger
	client   client.HTTPGetter
	nodeName string
}

// KubeletFlags represents key kubelet command-line flags we want to track.
type KubeletFlags struct {
	// Server Settings
	Address      string
	Port         int32
	ReadOnlyPort int32

	// Security Settings
	AnonymousAuth                        bool
	AuthorizationMode                    string
	ClientCAFile                         string
	TLSCertFile                          string
	TLSPrivateKeyFile                    string
	RotateCertificates                   bool
	ServerTLSBootstrap                   bool

	// Resource Management
	MaxPods                              int32
	PodPidsLimit                         int64
	KubeReserved                         string
	SystemReserved                       string
	EvictionHard                         string
	EvictionSoft                         string

	// Container Runtime
	ContainerRuntime                     string
	ContainerRuntimeEndpoint             string
	ImageServiceEndpoint                 string
	CgroupDriver                         string
	CgroupRoot                           string
	RuntimeRequestTimeout                string

	// Pod Management
	PodManifestPath                      string
	ManifestURL                          string
	ManifestURLHeader                    string
	SyncFrequency                        string
	FileCheckFrequency                   string
	HTTPCheckFrequency                   string

	// Networking
	ClusterDNS                           string
	ClusterDomain                        string
	NetworkPlugin                        string
	CNIBinDir                            string
	CNIConfDir                           string
	PodCIDR                              string
	NodeIP                               string

	// Feature Gates
	FeatureGates                         string

	// Node Management
	RegisterNode                         bool
	RegisterSchedulable                  bool
	NodeLabels                           string
	NodeStatusUpdateFrequency            string
	NodeStatusReportFrequency            string

	// Housekeeping
	HousekeepingInterval                 string
	ImageGCHighThresholdPercent          int32
	ImageGCLowThresholdPercent           int32
	ImageMinimumGCAge                    string

	// CPU/Memory Management
	CPUManagerPolicy                     string
	CPUManagerReconcilePeriod            string
	MemoryManagerPolicy                  string
	TopologyManagerPolicy                string
	TopologyManagerScope                 string
	ReservedSystemCPUs                   string

	// Logging & Monitoring
	LogLevel                             string
	V                                    string
	VModule                              string

	// Cloud Provider
	CloudProvider                        string
	CloudConfig                          string

	// Deprecated/Security Risk Flags (for alerting)
	EnableDebuggingHandlers              bool
	EnableContentionProfiling            bool
	AllowPrivileged                      bool
	HostnameOverride                     string
}

// NewKubeletFlagsFetcher creates a new KubeletFlagsFetcher
func NewKubeletFlagsFetcher(logger *log.Logger, client client.HTTPGetter, nodeName string) *KubeletFlagsFetcher {
	return &KubeletFlagsFetcher{
		logger:   logger,
		client:   client,
		nodeName: nodeName,
	}
}

// Fetch retrieves the kubelet flags from the /flags endpoint and returns them as RawGroups
func (f *KubeletFlagsFetcher) Fetch() (definition.RawGroups, error) {
	f.logger.Debugf("Fetching kubelet flags from %s", FlagsPath)

	var resp *http.Response
	var err error

	// Use content negotiation to request text/plain format (flags always returns text)
	if clientWithAccept, ok := f.client.(client.HTTPGetterWithAccept); ok {
		resp, err = clientWithAccept.GetWithAccept(FlagsPath, "text/plain")
	} else {
		resp, err = f.client.Get(FlagsPath)
	}
	if err != nil {
		return nil, fmt.Errorf("error calling kubelet %s path: %w", FlagsPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kubelet %s returned status %d: %s", FlagsPath, resp.StatusCode, string(body))
	}

	// Verify Content-Type (flags always returns text/plain)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "text/plain") {
		f.logger.Debugf("Unexpected Content-Type from %s: %s (expected text/plain)", FlagsPath, contentType)
	}

	flags, err := f.parseFlags(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing kubelet flags: %w", err)
	}

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

// parseFlags parses the plain text flags output from kubelet
func (f *KubeletFlagsFetcher) parseFlags(body io.Reader) (*KubeletFlags, error) {
	flags := &KubeletFlags{}
	flagMap := make(map[string]string)

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "--") {
			continue
		}

		// Remove leading --
		line = strings.TrimPrefix(line, "--")

		// Split on first =
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Flag without value, treat as boolean true
			flagMap[parts[0]] = "true"
			continue
		}

		flagName := parts[0]
		flagValue := parts[1]
		flagMap[flagName] = flagValue
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading flags: %w", err)
	}

	// Parse into struct
	f.parseIntoStruct(flagMap, flags)

	return flags, nil
}

// parseIntoStruct extracts known flags from the map into the struct
func (f *KubeletFlagsFetcher) parseIntoStruct(flagMap map[string]string, flags *KubeletFlags) {
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

// parseBool parses a string boolean with a default value
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

// flagsToRawMetrics converts KubeletFlags to RawMetrics
func (f *KubeletFlagsFetcher) flagsToRawMetrics(flags *KubeletFlags) (definition.RawMetrics, error) {
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

// calculateFlagsFingerprint generates a SHA256 hash of the flags for drift detection
func (f *KubeletFlagsFetcher) calculateFlagsFingerprint(flags *KubeletFlags) (string, error) {
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
