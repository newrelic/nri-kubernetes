package prober

import (
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
	Logger  *log.Logger
}

var ErrProbeTimeout = errors.New("probe timed out")
var errProbeNotOk = errors.New("probe did not return 200 Ok")

// New creates a Prober that will check an endpoint every backoff seconds.
func New(timeout, backoff time.Duration) *Prober {
	return &Prober{
		timeout: timeout,
		backoff: backoff,
		Logger:  logutil.Discard,
	}
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
			p.Logger.Debug(err)
			continue
		}

		return nil
	}
}

// attempt makes a request to the specified URL and returns an error if it does not return 200.
func (p *Prober) attempt(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("probe attempt to %s failed, retrying in %s: %w", url, p.backoff, err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", errProbeNotOk, resp.StatusCode)
	}

	return nil
}
