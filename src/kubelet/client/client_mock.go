package client

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"net/http"
	"net/url"
	"path"
)

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type Mock struct {
	// TODO: Use a non-sdk logger
	logger   log.Logger
	doer     client.HTTPDoer
	endpoint url.URL
}

func NewClientMock(doer client.HTTPDoer, endpoint url.URL) *Mock {
	return &Mock{
		logger:   log.NewStdErr(true),
		doer:     doer,
		endpoint: endpoint,
	}
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (c *Mock) Get(urlPath string) (*http.Response, error) {
	// Notice that this is the client to interact with kubelet. In case of CAdvisor the prometheus.Do is used

	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := http.NewRequest(http.MethodGet, e.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %s ", e.String(), err)
	}

	return c.doer.Do(r)
}

// MetricFamiliesGetFunc returns a function that obtains metric families from a list of prometheus queries.
func (c *Mock) MetricFamiliesGetFunc(url string) prometheus.FetchAndFilterMetricsFamilies {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, url, queries, c.logger)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families %q: %w", url, err)
		}

		return mFamily, nil
	}
}
