package client

import (
	"fmt"
	"time"

	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
)

// Client implements a client for KSM, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	// http is an HttpDoer that the KSM client will use to make requests.
	http    client.HTTPDoer
	logger  *log.Logger
	retries int
	timeout time.Duration
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kc *Client) error {
		kc.logger = logger
		return nil
	}
}

// WithTimeout returns an OptionFunc to change the timeout for Pester Client.
func WithTimeout(timeout time.Duration) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.timeout = timeout
		return nil
	}
}

// WithMaxRetries returns an OptionFunc to change the number of retries for Pester Client.
func WithMaxRetries(retries int) OptionFunc {
	return func(kubeletClient *Client) error {
		kubeletClient.retries = retries
		return nil
	}
}

// New builds a Client using the given options. By default, it will use pester as an HTTP Doer and a noop logger.
func New(opts ...OptionFunc) (*Client, error) {
	k := &Client{
		logger: logutil.Discard,
	}

	for i, opt := range opts {
		if err := opt(k); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	httpPester := pester.New()
	httpPester.Backoff = pester.LinearBackoff
	httpPester.MaxRetries = k.retries
	httpPester.Timeout = k.timeout
	httpPester.LogHook = func(e pester.ErrEntry) {
		k.logger.Debugf("getting data from ksm: %v", e)
	}
	k.http = httpPester

	return k, nil
}

// MetricFamiliesGetFunc returns a function that obtains metric families from a list of prometheus queries.
func (c *Client) MetricFamiliesGetFunc(url string) prometheus.FetchAndFilterMetricsFamilies {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.http, url, queries, c.logger)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}
