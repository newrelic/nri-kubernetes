package client

import (
	"fmt"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"io"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"

	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// Client implements a client for KSM, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	// http is an HttpDoer that the KSM client will use to make requests.
	http client.HTTPDoer
	// TODO: Use a non-sdk logger
	logger log.Logger
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(kc *Client) error {
		kc.logger = logger
		return nil
	}
}

// New builds a Client using the given options. By default, it will use pester as an HTTP Doer and a noop logger.
func New(opts ...OptionFunc) (*Client, error) {
	k := &Client{
		logger: log.New(false, io.Discard),
	}

	httpPester := pester.New()
	httpPester.Backoff = pester.LinearBackoff
	httpPester.MaxRetries = 3
	httpPester.Timeout = 10 * time.Second
	httpPester.LogHook = func(e pester.ErrEntry) {
		k.logger.Debugf("getting data from ksm: %v", e)
	}
	k.http = httpPester

	for i, opt := range opts {
		if err := opt(k); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

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
