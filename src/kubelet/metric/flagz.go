package metric

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/definition"
)

const (
	// FlagzPath is the path where kubelet exposes its command-line flags.
	// Requires ComponentFlagz feature gate (alpha in 1.32+).
	FlagzPath = "/flagz"
)

// KubeletFlagzFetcher queries the kubelet /flagz endpoint to fetch command-line flags.
// The /flagz endpoint requires the ComponentFlagz feature gate to be enabled.
// It returns plain text key=value format (same as the legacy /flags endpoint).
type KubeletFlagzFetcher struct {
	logger   *log.Logger
	client   client.HTTPGetter
	nodeName string
	parser   *FlagsParser
}

// NewKubeletFlagzFetcher creates a new KubeletFlagzFetcher.
func NewKubeletFlagzFetcher(logger *log.Logger, client client.HTTPGetter, nodeName string) *KubeletFlagzFetcher {
	return &KubeletFlagzFetcher{
		logger:   logger,
		client:   client,
		nodeName: nodeName,
		parser:   NewFlagsParser(logger),
	}
}

// Fetch retrieves the kubelet flags from the /flagz endpoint and returns them as RawGroups.
// The /flagz endpoint returns plain text key=value format.
func (f *KubeletFlagzFetcher) Fetch() (definition.RawGroups, error) {
	f.logger.Debugf("Fetching kubelet flags from %s", FlagzPath)

	var resp *http.Response
	var err error

	// Use content negotiation to request text/plain format (flagz always returns text).
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
		return nil, fmt.Errorf("%w: kubelet %s returned status %d: %s", ErrHTTPStatusError, FlagzPath, resp.StatusCode, string(body))
	}

	// Read response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from kubelet %s: %w", FlagzPath, err)
	}

	// Verify Content-Type (flagz always returns text/plain).
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "text/plain") {
		f.logger.Debugf("Unexpected Content-Type from %s: %s (expected text/plain)", FlagzPath, contentType)
	}

	// Parse plain text key=value format (same format as /flags).
	flagMap := f.parsePlainTextFlags(string(body))

	// Convert to KubeletFlags structure.
	flags := &KubeletFlags{}
	f.parser.ParseIntoStruct(flagMap, flags)

	// Convert flags to RawMetrics.
	rawMetrics := f.parser.FlagsToRawMetrics(flags)

	rawGroups := definition.RawGroups{
		"node": {
			f.nodeName: rawMetrics,
		},
	}

	return rawGroups, nil
}

// parsePlainTextFlags parses the plain text key=value format from /flagz.
// Format: "flag=value" lines (without leading --).
func (f *KubeletFlagzFetcher) parsePlainTextFlags(body string) map[string]string {
	flagMap := make(map[string]string)

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines, headers, and warnings.
		if line == "" || strings.HasPrefix(line, "kubelet") || strings.HasPrefix(line, "Warning:") {
			continue
		}

		// Split on first =.
		parts := strings.SplitN(line, "=", flagsSplitParts)
		if len(parts) != flagsSplitParts {
			continue
		}

		flagName := strings.TrimSpace(parts[0])
		flagValue := strings.TrimSpace(parts[1])
		flagMap[flagName] = flagValue
	}

	return flagMap
}
