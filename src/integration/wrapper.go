package integration

import (
	"fmt"
	"net"
	"strconv"
	"time"

	sdk "github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"
	"github.com/newrelic/nri-kubernetes/v3/src/integration/prober"
	"github.com/newrelic/nri-kubernetes/v3/src/integration/sink"
	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
)

const defaultProbeTimeout = 90 * time.Second
const defaultProbeBackoff = 5 * time.Second

const agentReadyPath = "/v1/data/ready"

// Wrapper is a wrapper on top of the SDK integration.
type Wrapper struct {
	sdkIntegration *sdk.Integration
	logger         *log.Logger
	metadata       Metadata
	probeTimeout   time.Duration
	probeBackoff   time.Duration
}

// OptionFunc is an option func for the Wrapper.
type OptionFunc func(i *Wrapper) error

func WithLogger(logger *log.Logger) OptionFunc {
	return func(i *Wrapper) error {
		i.logger = logger
		return nil
	}
}

// WithProbeTimeout configures the integration wrapper to wait at most timeout for the HTTP endpoint to respond.
func WithProbeTimeout(timeout time.Duration) OptionFunc {
	return func(i *Wrapper) error {
		i.probeTimeout = timeout
		return nil
	}
}

// WithProbeBackoff configures the time the internal prober waits between HTTP endpoint checks.
func WithProbeBackoff(backoff time.Duration) OptionFunc {
	return func(i *Wrapper) error {
		i.probeBackoff = backoff
		return nil
	}
}

// WithMetadata allows to configure the integration name and version that is passed down to the integration SDK.
func WithMetadata(metadata Metadata) OptionFunc {
	return func(i *Wrapper) error {
		i.metadata = metadata
		return nil
	}
}

// Metadata contains the integration name and version that is passed down to the integration SDK.
type Metadata struct {
	Name    string
	Version string
}

// NewWrapper creates a new SDK integration wrapper using the specified options.
func NewWrapper(opts ...OptionFunc) (*Wrapper, error) {
	intgr := &Wrapper{
		logger:       logutil.Discard,
		probeTimeout: defaultProbeTimeout,
		probeBackoff: defaultProbeBackoff,
	}

	for _, opt := range opts {
		err := opt(intgr)
		if err != nil {
			return nil, fmt.Errorf("applying option: %w", err)
		}
	}

	return intgr, nil
}

// Integration returns a sdk.Integration, configured to output data to the specified agent.
// Integration will block and wait until the specified server is ready, up to a maximum timeout.
func (iw *Wrapper) Integration(sinkConfig config.HTTPSink) (*sdk.Integration, error) {
	hostPort := net.JoinHostPort(sink.DefaultAgentForwarderhost, strconv.Itoa(sinkConfig.Port))

	prober := prober.New(iw.probeTimeout, iw.probeBackoff)
	iw.logger.Info("Waiting for agent container to be ready...")

	err := prober.Probe(fmt.Sprintf("http://%s%s", hostPort, agentReadyPath))
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for agent: %w", err)
	}

	c := pester.New()
	c.Backoff = pester.LinearBackoff
	c.MaxRetries = sinkConfig.Retries
	c.Timeout = sinkConfig.Timeout
	c.LogHook = func(e pester.ErrEntry) {
		// LogHook is invoked only when an error happens
		iw.logger.Warnf("Error sending data to agent sink: %q", e)
	}

	h, err := sink.New(sink.HTTPSinkOptions{
		URL:    fmt.Sprintf("http://%s%s", hostPort, sink.DefaultAgentForwarderPath),
		Client: c,
	})
	if err != nil {
		return nil, fmt.Errorf("creating HTTPSink: %w", err)
	}

	cache := storer.NewInMemoryStore(storer.DefaultTTL, storer.DefaultInterval, iw.logger)
	return sdk.New(iw.metadata.Name, iw.metadata.Version, sdk.Writer(h), sdk.Storer(cache))
}
