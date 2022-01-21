package integration

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	sdk "github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/internal/storer"
	"github.com/newrelic/nri-kubernetes/v3/src/integration/prober"
	"github.com/newrelic/nri-kubernetes/v3/src/integration/sink"
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
	sink           io.Writer
}

// OptionFunc is an option func for the Wrapper.
type OptionFunc func(i *Wrapper) error

func WithLogger(logger *log.Logger) OptionFunc {
	return func(i *Wrapper) error {
		i.logger = logger
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

// WithHTTPSink configures the wrapper to use an HTTP Sink for metrics.
// If this option is not specified, Wrapper will configure the integration.Integration to sink metrics to stdout.
func WithHTTPSink(sinkConfig config.HTTPSink) OptionFunc {
	return func(iw *Wrapper) error {
		hostPort := net.JoinHostPort(sink.DefaultAgentForwarderhost, strconv.Itoa(sinkConfig.Port))

		prober := prober.New(iw.probeTimeout, iw.probeBackoff)
		iw.logger.Info("Waiting for agent container to be ready...")

		err := prober.Probe(fmt.Sprintf("http://%s%s", hostPort, agentReadyPath))
		if err != nil {
			return fmt.Errorf("timeout waiting for agent: %w", err)
		}

		c := pester.New()
		if sinkConfig.TLS.Enabled {
			tlsClient, err := sink.NewTLSClient(sinkConfig.TLS)
			if err != nil {
				return fmt.Errorf("creating TLS client: %w", err)
			}

			c.EmbedHTTPClient(tlsClient)
		}

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
			return fmt.Errorf("creating HTTP Sink: %w", err)
		}

		iw.sink = h
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
		sink:         os.Stdout,
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
func (iw *Wrapper) Integration() (*sdk.Integration, error) {
	cache := storer.NewInMemoryStore(storer.DefaultTTL, storer.DefaultInterval, iw.logger)
	return sdk.New(iw.metadata.Name, iw.metadata.Version, sdk.Writer(iw.sink), sdk.Storer(cache))
}
