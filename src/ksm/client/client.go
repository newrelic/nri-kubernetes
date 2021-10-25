package client

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// ksm implements Client interface
type ksm struct {
	httpClient *http.Client
	endpoint   url.URL
	nodeIP     string
	logger     log.Logger
}

type KSMClient interface {
	MetricFamiliesGetterForEndpoint(endpoint string) prometheus.FilteredMetricFamilies
}

func NewKSMClient(timeout time.Duration, logger log.Logger) (KSMClient, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger not provided")
	}

	return &ksm{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}, nil
}

func (c *ksm) MetricFamiliesGetterForEndpoint(endpoint string) prometheus.FilteredMetricFamilies {
	ksmMetricsURL := url.URL{
		Scheme: "http",
		Host:   endpoint,
		Path:   metric.PrometheusMetricsPath,
	}

	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c, ksmMetricsURL.String(), queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}

func (c *ksm) Do(r *http.Request) (*http.Response, error) {
	c.logger.Debugf("Calling kube-state-metrics endpoint: %s", r.URL.String())

	// Calls http.Client.
	return c.httpClient.Do(r)
}

func (c *ksm) NodeIP() string {
	return c.nodeIP
}

// Get implements HTTPGetter interface by sending Prometheus plain text request using configured client.
func (c *ksm) Get(urlPath string) (*http.Response, error) {
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	url := e.String()

	// Creates Prometheus request.
	r, err := prometheus.NewRequest(url)
	if err != nil {
		return nil, fmt.Errorf("Error creating request to: %s. Got error: %s ", url, err)
	}

	return c.Do(r)
}

func newKSMClient(timeout time.Duration, nodeIP string, endpoint url.URL, logger log.Logger) *ksm {
	return &ksm{
		nodeIP:   nodeIP,
		endpoint: endpoint,
		// HTTP Client definition.
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}
