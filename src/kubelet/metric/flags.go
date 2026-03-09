package metric

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	// FlagsPath is the path where kubelet exposes its command-line flags.
	FlagsPath = "/flags"
)

// KubeletFlagsFetcher queries the kubelet /flags endpoint to fetch command-line flags.
type KubeletFlagsFetcher struct {
	logger   *log.Logger
	client   client.HTTPGetter
	nodeName string
	parser   *FlagsParser
}

// KubeletFlags represents key kubelet command-line flags we want to track.
type KubeletFlags struct {
	// Server Settings
	Address      string
	Port         int32
	ReadOnlyPort int32

	// Security Settings
	AnonymousAuth      bool
	AuthorizationMode  string
	ClientCAFile       string
	TLSCertFile        string
	TLSPrivateKeyFile  string
	RotateCertificates bool
	ServerTLSBootstrap bool

	// Resource Management
	MaxPods        int32
	PodPidsLimit   int64
	KubeReserved   string
	SystemReserved string
	EvictionHard   string
	EvictionSoft   string

	// Container Runtime
	ContainerRuntime         string
	ContainerRuntimeEndpoint string
	ImageServiceEndpoint     string
	CgroupDriver             string
	CgroupRoot               string
	RuntimeRequestTimeout    string

	// Pod Management
	PodManifestPath    string
	ManifestURL        string
	ManifestURLHeader  string
	SyncFrequency      string
	FileCheckFrequency string
	HTTPCheckFrequency string

	// Networking
	ClusterDNS    string
	ClusterDomain string
	NetworkPlugin string
	CNIBinDir     string
	CNIConfDir    string
	PodCIDR       string
	NodeIP        string

	// Feature Gates
	FeatureGates string

	// Node Management
	RegisterNode              bool
	RegisterSchedulable       bool
	NodeLabels                string
	NodeStatusUpdateFrequency string
	NodeStatusReportFrequency string

	// Housekeeping
	HousekeepingInterval        string
	ImageGCHighThresholdPercent int32
	ImageGCLowThresholdPercent  int32
	ImageMinimumGCAge           string

	// CPU/Memory Management
	CPUManagerPolicy          string
	CPUManagerReconcilePeriod string
	MemoryManagerPolicy       string
	TopologyManagerPolicy     string
	TopologyManagerScope      string
	ReservedSystemCPUs        string

	// Logging & Monitoring
	LogLevel string
	V        string
	VModule  string

	// Cloud Provider
	CloudProvider string
	CloudConfig   string

	// Deprecated/Security Risk Flags (for alerting)
	EnableDebuggingHandlers   bool
	EnableContentionProfiling bool
	AllowPrivileged           bool
	HostnameOverride          string
}

// NewKubeletFlagsFetcher creates a new KubeletFlagsFetcher.
func NewKubeletFlagsFetcher(logger *log.Logger, client client.HTTPGetter, nodeName string) *KubeletFlagsFetcher {
	return &KubeletFlagsFetcher{
		logger:   logger,
		client:   client,
		nodeName: nodeName,
		parser:   NewFlagsParser(logger),
	}
}

// Fetch retrieves the kubelet flags from the /flags endpoint and returns them as RawGroups.
func (f *KubeletFlagsFetcher) Fetch() (definition.RawGroups, error) {
	f.logger.Debugf("Fetching kubelet flags from %s", FlagsPath)

	var resp *http.Response
	var err error

	// Use content negotiation to request text/plain format (flags always returns text).
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
		return nil, fmt.Errorf("%w: kubelet %s returned status %d: %s", ErrHTTPStatusError, FlagsPath, resp.StatusCode, string(body))
	}

	// Verify Content-Type (flags always returns text/plain).
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "text/plain") {
		f.logger.Debugf("Unexpected Content-Type from %s: %s (expected text/plain)", FlagsPath, contentType)
	}

	flags, err := f.parseFlags(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing kubelet flags: %w", err)
	}

	// Convert flags to RawMetrics
	rawMetrics := f.parser.FlagsToRawMetrics(flags)

	rawGroups := definition.RawGroups{
		"node": {
			f.nodeName: rawMetrics,
		},
	}

	return rawGroups, nil
}

// parseFlags parses the plain text flags output from kubelet.
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
		parts := strings.SplitN(line, "=", flagsSplitParts)
		if len(parts) != flagsSplitParts {
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
	f.parser.ParseIntoStruct(flagMap, flags)

	return flags, nil
}

// calculateFlagsFingerprint generates a SHA256 hash of the flags for drift detection.
func (f *KubeletFlagsFetcher) calculateFlagsFingerprint(flags *KubeletFlags) string {
	return f.parser.CalculateFlagsFingerprint(flags)
}

// flagsToRawMetrics converts KubeletFlags to RawMetrics.
func (f *KubeletFlagsFetcher) flagsToRawMetrics(flags *KubeletFlags) definition.RawMetrics {
	return f.parser.FlagsToRawMetrics(flags)
}
