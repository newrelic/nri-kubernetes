package prober

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
)

// Prober is an object that polls and http URL and returns an error if it does not return 200 Ok within the specified
// timeout.
type Prober struct {
	timeout time.Duration
	backoff time.Duration
	logger  *log.Logger
	client  *http.Client
}

var ErrProbeTimeout = errors.New("probe timed out")
var errProbeNotOk = errors.New("probe did not return 200 Ok")

type OptionFunc func(p *Prober) error

// WithLogger returns an OptionFunc which tells the Prober to use the specified logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(p *Prober) error {
		p.logger = logger
		return nil
	}
}

// WithClient returns an OptionFunc which tells the Prober to use the specified client for probing.
func WithClient(client *http.Client) OptionFunc {
	return func(p *Prober) error {
		p.client = client
		return nil
	}
}

// New creates a Prober that will check an endpoint every backoff seconds.
func New(timeout, backoff time.Duration, options ...OptionFunc) (*Prober, error) {
	p := &Prober{
		timeout: timeout,
		backoff: backoff,
		logger:  logutil.Discard,
		client:  http.DefaultClient,
	}

	for _, opt := range options {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("configuring prober: %w", err)
		}
	}

	return p, nil
}

// Probe repeatedly hits the specified url with a GET request every Prober.backoff, and blocks until a request returns
// 200, or Prober.timeout passes.
func (p *Prober) Probe(url string) error {
	start := time.Now()
	for {
		if time.Since(start) > p.timeout {
			return fmt.Errorf("%w after %s", ErrProbeTimeout, p.timeout)
		}

		err := p.attempt(url)
		if err != nil {
			p.logger.Debug(err)
			p.logger.Debugf("Retrying in %s", p.backoff)
			time.Sleep(p.backoff)
			continue
		}

		return nil
	}
}

// attempt makes a request to the specified URL and returns an error if it does not return 200.
func (p *Prober) attempt(url string) error {
	// As the prober can use a custom HTTP client with an independent, potentially unbound timeout, we need to ensure
	// that requests will not hang forever, hence the need of a custom context.
	// requestTimeout is set to the global timeout /3 to guarantee that we will be able to probe at least twice within
	// the global timeout, so a stuck request does not eat the whole timeout and cause a probe to fail.
	requestTimeout := p.timeout / 3
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building GET %q: %w", url, err)
	}

	resp, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("probe attempt to infra agent (%s) failed: %w", url, err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", errProbeNotOk, resp.StatusCode)
	}

	return nil
}
