package metric

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	// ConfigzPath is the path where kubelet exposes its configuration
	ConfigzPath = "/configz"
)

// KubeletConfigFetcher queries the kubelet /configz endpoint to fetch configuration.
type KubeletConfigFetcher struct {
	logger   *log.Logger
	client   client.HTTPGetter
	nodeName string
}

// KubeletConfigSnapshot represents key kubelet configuration settings we want to track.
// This is a subset of the full kubelet configuration focused on operationally important settings.
type KubeletConfigSnapshot struct {
	// Resource Management
	MaxPods                      *int32  `json:"maxPods,omitempty"`
	PodPidsLimit                 *int64  `json:"podPidsLimit,omitempty"`
	PodsPerCore                  *int32  `json:"podsPerCore,omitempty"`
	ImageGCHighThresholdPercent  *int32  `json:"imageGCHighThresholdPercent,omitempty"`
	ImageGCLowThresholdPercent   *int32  `json:"imageGCLowThresholdPercent,omitempty"`
	ImageMinimumGCAge            *string `json:"imageMinimumGCAge,omitempty"`
	ContainerLogMaxSize          *string `json:"containerLogMaxSize,omitempty"`
	ContainerLogMaxFiles         *int32  `json:"containerLogMaxFiles,omitempty"`

	// Eviction Configuration
	EvictionHard                map[string]string `json:"evictionHard,omitempty"`
	EvictionSoft                map[string]string `json:"evictionSoft,omitempty"`
	EvictionSoftGracePeriod     map[string]string `json:"evictionSoftGracePeriod,omitempty"`
	EvictionPressureTransitionPeriod *string      `json:"evictionPressureTransitionPeriod,omitempty"`
	EvictionMaxPodGracePeriod   *int32            `json:"evictionMaxPodGracePeriod,omitempty"`
	EvictionMinimumReclaim      map[string]string `json:"evictionMinimumReclaim,omitempty"`

	// QoS and Resource Management Policies
	CPUManagerPolicy          *string           `json:"cpuManagerPolicy,omitempty"`
	CPUManagerPolicyOptions   map[string]string `json:"cpuManagerPolicyOptions,omitempty"`
	CPUManagerReconcilePeriod *string           `json:"cpuManagerReconcilePeriod,omitempty"`
	MemoryManagerPolicy       *string           `json:"memoryManagerPolicy,omitempty"`
	TopologyManagerPolicy     *string           `json:"topologyManagerPolicy,omitempty"`
	TopologyManagerScope      *string           `json:"topologyManagerScope,omitempty"`
	QOSReserved               map[string]string `json:"qosReserved,omitempty"`

	// Reserved Resources
	KubeReserved       map[string]string `json:"kubeReserved,omitempty"`
	SystemReserved     map[string]string `json:"systemReserved,omitempty"`
	ReservedSystemCPUs *string           `json:"reservedSystemCPUs,omitempty"`

	// Security Settings
	ProtectKernelDefaults     *bool   `json:"protectKernelDefaults,omitempty"`
	SeccompDefault            *bool   `json:"seccompDefault,omitempty"`
	AllowedUnsafeSysctls      []string `json:"allowedUnsafeSysctls,omitempty"`
	EnableDebuggingHandlers   *bool   `json:"enableDebuggingHandlers,omitempty"`
	EnableContentionProfiling *bool   `json:"enableContentionProfiling,omitempty"`

	// Authentication & Authorization
	Authentication *KubeletAuthentication `json:"authentication,omitempty"`
	Authorization  *KubeletAuthorization  `json:"authorization,omitempty"`

	// Feature Gates
	FeatureGates map[string]bool `json:"featureGates,omitempty"`

	// Networking
	ClusterDNS            []string `json:"clusterDNS,omitempty"`
	ClusterDomain         *string  `json:"clusterDomain,omitempty"`
	ResolverConfig        *string  `json:"resolverConfig,omitempty"`
	HairpinMode           *string  `json:"hairpinMode,omitempty"`
	MaxOpenFiles          *int64   `json:"maxOpenFiles,omitempty"`
	MaxPerPodContainerCount *int64 `json:"maxPerPodContainerCount,omitempty"`

	// Runtime
	ContainerRuntimeEndpoint *string `json:"containerRuntimeEndpoint,omitempty"`
	ImageServiceEndpoint     *string `json:"imageServiceEndpoint,omitempty"`
	RuntimeRequestTimeout    *string `json:"runtimeRequestTimeout,omitempty"`
	CgroupDriver             *string `json:"cgroupDriver,omitempty"`
	CgroupRoot               *string `json:"cgroupRoot,omitempty"`
	CgroupsPerQOS            *bool   `json:"cgroupsPerQOS,omitempty"`

	// Logging
	Logging *KubeletLogging `json:"logging,omitempty"`

	// Shutdown
	ShutdownGracePeriod             *string `json:"shutdownGracePeriod,omitempty"`
	ShutdownGracePeriodCriticalPods *string `json:"shutdownGracePeriodCriticalPods,omitempty"`

	// Memory Management
	MemoryThrottlingFactor *float64 `json:"memoryThrottlingFactor,omitempty"`
	MemorySwap             *MemorySwapConfiguration `json:"memorySwap,omitempty"`

	// Server Settings
	Address                    *string `json:"address,omitempty"`
	Port                       *int32  `json:"port,omitempty"`
	ReadOnlyPort               *int32  `json:"readOnlyPort,omitempty"`
	TLSCertFile                *string `json:"tlsCertFile,omitempty"`
	TLSPrivateKeyFile          *string `json:"tlsPrivateKeyFile,omitempty"`
	TLSCipherSuites            []string `json:"tlsCipherSuites,omitempty"`
	TLSMinVersion              *string `json:"tlsMinVersion,omitempty"`
	ServerTLSBootstrap         *bool   `json:"serverTLSBootstrap,omitempty"`
}

// KubeletAuthentication contains authentication configuration
type KubeletAuthentication struct {
	X509        *KubeletX509        `json:"x509,omitempty"`
	Webhook     *KubeletWebhook     `json:"webhook,omitempty"`
	Anonymous   *KubeletAnonymous   `json:"anonymous,omitempty"`
}

type KubeletX509 struct {
	ClientCAFile *string `json:"clientCAFile,omitempty"`
}

type KubeletWebhook struct {
	Enabled      *bool   `json:"enabled,omitempty"`
	CacheTTL     *string `json:"cacheTTL,omitempty"`
}

type KubeletAnonymous struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// KubeletAuthorization contains authorization configuration
type KubeletAuthorization struct {
	Mode    *string         `json:"mode,omitempty"`
	Webhook *KubeletWebhook `json:"webhook,omitempty"`
}

// KubeletLogging contains logging configuration
type KubeletLogging struct {
	Format           *string `json:"format,omitempty"`
	FlushFrequency   *string `json:"flushFrequency,omitempty"`
	Verbosity        *int32  `json:"verbosity,omitempty"`
	VModule          *string `json:"vmodule,omitempty"`
}

// MemorySwapConfiguration contains memory swap settings
type MemorySwapConfiguration struct {
	SwapBehavior *string `json:"swapBehavior,omitempty"`
}

// kubeletConfigResponse represents the JSON structure returned by /configz
type kubeletConfigResponse struct {
	ComponentConfig KubeletConfigSnapshot `json:"kubeletconfig"`
}

// NewKubeletConfigFetcher creates a new KubeletConfigFetcher
func NewKubeletConfigFetcher(logger *log.Logger, client client.HTTPGetter, nodeName string) *KubeletConfigFetcher {
	return &KubeletConfigFetcher{
		logger:   logger,
		client:   client,
		nodeName: nodeName,
	}
}

// Fetch retrieves the kubelet configuration from the /configz endpoint and returns it as RawGroups
func (f *KubeletConfigFetcher) Fetch() (definition.RawGroups, error) {
	f.logger.Debugf("Fetching kubelet configuration from %s", ConfigzPath)

	var resp *http.Response
	var err error

	// Use content negotiation to request JSON format
	if clientWithAccept, ok := f.client.(client.HTTPGetterWithAccept); ok {
		resp, err = clientWithAccept.GetWithAccept(ConfigzPath, "application/json")
	} else {
		resp, err = f.client.Get(ConfigzPath)
	}
	if err != nil {
		return nil, fmt.Errorf("error calling kubelet %s path: %w", ConfigzPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kubelet %s returned status %d: %s", ConfigzPath, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from kubelet %s path: %w", ConfigzPath, err)
	}

	// Verify Content-Type is JSON (configz always returns JSON)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		f.logger.Warnf("Unexpected Content-Type from %s: %s (expected application/json)", ConfigzPath, contentType)
	}

	var configResp kubeletConfigResponse
	if err := json.Unmarshal(body, &configResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling kubelet config response: %w", err)
	}

	config := configResp.ComponentConfig

	// Convert config to RawMetrics
	rawMetrics, err := f.configToRawMetrics(&config)
	if err != nil {
		return nil, fmt.Errorf("error converting config to raw metrics: %w", err)
	}

	rawGroups := definition.RawGroups{
		"node": {
			f.nodeName: rawMetrics,
		},
	}

	return rawGroups, nil
}

// configToRawMetrics converts a KubeletConfigSnapshot to RawMetrics
func (f *KubeletConfigFetcher) configToRawMetrics(config *KubeletConfigSnapshot) (definition.RawMetrics, error) {
	metrics := make(definition.RawMetrics)

	// Add a configuration fingerprint for drift detection
	fingerprint, err := f.calculateConfigFingerprint(config)
	if err != nil {
		f.logger.Warnf("Failed to calculate config fingerprint: %v", err)
	} else {
		metrics["kubeletConfigFingerprint"] = fingerprint
	}

	// Resource Management
	if config.MaxPods != nil {
		metrics["kubeletMaxPods"] = *config.MaxPods
	}
	if config.PodPidsLimit != nil {
		metrics["kubeletPodPidsLimit"] = *config.PodPidsLimit
	}
	if config.ImageGCHighThresholdPercent != nil {
		metrics["kubeletImageGCHighThresholdPercent"] = *config.ImageGCHighThresholdPercent
	}
	if config.ImageGCLowThresholdPercent != nil {
		metrics["kubeletImageGCLowThresholdPercent"] = *config.ImageGCLowThresholdPercent
	}

	// Eviction - convert maps to JSON strings for storage
	if len(config.EvictionHard) > 0 {
		if evictionJSON, err := json.Marshal(config.EvictionHard); err == nil {
			metrics["kubeletEvictionHard"] = string(evictionJSON)
		}
	}
	if len(config.EvictionSoft) > 0 {
		if evictionJSON, err := json.Marshal(config.EvictionSoft); err == nil {
			metrics["kubeletEvictionSoft"] = string(evictionJSON)
		}
	}

	// QoS and Resource Policies
	if config.CPUManagerPolicy != nil {
		metrics["kubeletCPUManagerPolicy"] = *config.CPUManagerPolicy
	}
	if config.MemoryManagerPolicy != nil {
		metrics["kubeletMemoryManagerPolicy"] = *config.MemoryManagerPolicy
	}
	if config.TopologyManagerPolicy != nil {
		metrics["kubeletTopologyManagerPolicy"] = *config.TopologyManagerPolicy
	}
	if config.TopologyManagerScope != nil {
		metrics["kubeletTopologyManagerScope"] = *config.TopologyManagerScope
	}

	// Reserved Resources
	if len(config.KubeReserved) > 0 {
		if reservedJSON, err := json.Marshal(config.KubeReserved); err == nil {
			metrics["kubeletKubeReserved"] = string(reservedJSON)
		}
	}
	if len(config.SystemReserved) > 0 {
		if reservedJSON, err := json.Marshal(config.SystemReserved); err == nil {
			metrics["kubeletSystemReserved"] = string(reservedJSON)
		}
	}
	if config.ReservedSystemCPUs != nil {
		metrics["kubeletReservedSystemCPUs"] = *config.ReservedSystemCPUs
	}

	// Security Settings
	if config.ProtectKernelDefaults != nil {
		metrics["kubeletProtectKernelDefaults"] = *config.ProtectKernelDefaults
	}
	if config.SeccompDefault != nil {
		metrics["kubeletSeccompDefault"] = *config.SeccompDefault
	}
	if config.EnableDebuggingHandlers != nil {
		metrics["kubeletEnableDebuggingHandlers"] = *config.EnableDebuggingHandlers
	}

	// Authentication & Authorization
	if config.Authentication != nil && config.Authentication.Anonymous != nil && config.Authentication.Anonymous.Enabled != nil {
		metrics["kubeletAnonymousAuthEnabled"] = *config.Authentication.Anonymous.Enabled
	}
	if config.Authentication != nil && config.Authentication.Webhook != nil && config.Authentication.Webhook.Enabled != nil {
		metrics["kubeletWebhookAuthEnabled"] = *config.Authentication.Webhook.Enabled
	}
	if config.Authorization != nil && config.Authorization.Mode != nil {
		metrics["kubeletAuthorizationMode"] = *config.Authorization.Mode
	}

	// Feature Gates - convert to JSON string
	if len(config.FeatureGates) > 0 {
		if gatesJSON, err := json.Marshal(config.FeatureGates); err == nil {
			metrics["kubeletFeatureGates"] = string(gatesJSON)
		}
		// Also count how many feature gates are enabled
		enabledCount := 0
		for _, enabled := range config.FeatureGates {
			if enabled {
				enabledCount++
			}
		}
		metrics["kubeletFeatureGatesEnabledCount"] = enabledCount
	}

	// Networking
	if len(config.ClusterDNS) > 0 {
		if dnsJSON, err := json.Marshal(config.ClusterDNS); err == nil {
			metrics["kubeletClusterDNS"] = string(dnsJSON)
		}
	}
	if config.ClusterDomain != nil {
		metrics["kubeletClusterDomain"] = *config.ClusterDomain
	}
	if config.HairpinMode != nil {
		metrics["kubeletHairpinMode"] = *config.HairpinMode
	}
	if config.MaxOpenFiles != nil {
		metrics["kubeletMaxOpenFiles"] = *config.MaxOpenFiles
	}

	// Runtime
	if config.ContainerRuntimeEndpoint != nil {
		metrics["kubeletContainerRuntimeEndpoint"] = *config.ContainerRuntimeEndpoint
	}
	if config.CgroupDriver != nil {
		metrics["kubeletCgroupDriver"] = *config.CgroupDriver
	}
	if config.CgroupsPerQOS != nil {
		metrics["kubeletCgroupsPerQOS"] = *config.CgroupsPerQOS
	}

	// Server Settings (Security-relevant)
	if config.Port != nil {
		metrics["kubeletPort"] = *config.Port
	}
	if config.ReadOnlyPort != nil {
		metrics["kubeletReadOnlyPort"] = *config.ReadOnlyPort
		// Flag if read-only port is enabled (security risk)
		metrics["kubeletReadOnlyPortEnabled"] = *config.ReadOnlyPort != 0
	}
	if config.ServerTLSBootstrap != nil {
		metrics["kubeletServerTLSBootstrap"] = *config.ServerTLSBootstrap
	}
	if config.TLSMinVersion != nil {
		metrics["kubeletTLSMinVersion"] = *config.TLSMinVersion
	}

	// Shutdown configuration
	if config.ShutdownGracePeriod != nil {
		metrics["kubeletShutdownGracePeriod"] = *config.ShutdownGracePeriod
	}
	if config.ShutdownGracePeriodCriticalPods != nil {
		metrics["kubeletShutdownGracePeriodCriticalPods"] = *config.ShutdownGracePeriodCriticalPods
	}

	// Memory settings
	if config.MemoryThrottlingFactor != nil {
		metrics["kubeletMemoryThrottlingFactor"] = *config.MemoryThrottlingFactor
	}
	if config.MemorySwap != nil && config.MemorySwap.SwapBehavior != nil {
		metrics["kubeletMemorySwapBehavior"] = *config.MemorySwap.SwapBehavior
	}

	// Store diagnostics map for wildcard metric expansion (PrefixFromMapAny transform)
	// This needs to be a map[string]interface{}, not a JSON string
	// Strip component-specific prefixes to get clean names like "Fingerprint", "MaxPods"
	configzDiagnostics := make(map[string]interface{})
	for k, v := range metrics {
		var key string
		switch {
		case len(k) > 13 && k[:13] == "kubeletConfig":
			key = k[13:] // kubeletConfigFingerprint -> Fingerprint
		case len(k) > 7 && k[:7] == "kubelet":
			key = k[7:] // kubeletMaxPods -> MaxPods
		default:
			key = k
		}
		configzDiagnostics[key] = v
	}
	metrics["kubeletConfigzDiagnostics"] = configzDiagnostics

	return metrics, nil
}

// calculateConfigFingerprint generates a SHA256 hash of the configuration for drift detection
func (f *KubeletConfigFetcher) calculateConfigFingerprint(config *KubeletConfigSnapshot) (string, error) {
	// Marshal the entire config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("error marshaling config for fingerprint: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(configJSON)
	fingerprint := fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes (16 hex chars) for brevity

	return fingerprint, nil
}
