package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/sethgrid/pester"

	"github.com/newrelic/nri-kubernetes/v2/src/ksm/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// Client implements a client for KSM, capable of retrieving prometheus metrics from a given endpoint.
type Client struct {
	// http is an HttpDoer that the KSM client will use to make requests.
	http HTTPDoer
	// TODO: Use a non-sdk logger
	logger log.Logger
}

// HTTPDoer is a simple interface encapsulating objects capable of making requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type OptionFunc func(kc *Client) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(kc *Client) error {
		kc.logger = logger
		return nil
	}
}

// WithHTTPDoer returns an OptionFunc that changes the HTTP Doer from the default Pester.
func WithHTTPDoer(doer HTTPDoer) OptionFunc {
	return func(kc *Client) error {
		kc.http = doer
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

// MetricFamiliesGetter is the interface satisfied by Client.
// TODO: This whole flow is too convoluted, we should refactor and rename this.
type MetricFamiliesGetter interface {
	// MetricFamiliesGetter returns a prometheus.FilteredFetcher configured to get KSM metrics from and endpoint.
	// prometheus.FilteredFetcher will be used by the prometheus client to scrape and filter metrics.
	MetricFamiliesGetter(endpoint string, schema string) prometheus.MetricsFamiliesGetter
}

// MetricFamiliesGetter returns a function that obtains metric families from a list of prometheus queries.
func (c *Client) MetricFamiliesGetter(endpoint string, schema string) prometheus.MetricsFamiliesGetter {
	ksmMetricsURL := url.URL{
		Scheme: schema,
		Host:   endpoint,
		Path:   metric.PrometheusMetricsPath,
	}

	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.http, ksmMetricsURL.String(), queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}
