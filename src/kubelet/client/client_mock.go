package client

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"net/http"
	"net/url"
	"path"
)

// Client implements a client for Kubelet, capable of retrieving prometheus metrics from a given endpoint.
type ClientMock struct {
	// TODO: Use a non-sdk logger
	logger   log.Logger
	doer     httpDoer
	endpoint url.URL
}

func NewClientMock(doer httpDoer, endpoint url.URL) *ClientMock {
	return &ClientMock{
		logger:   log.NewStdErr(true),
		doer:     doer,
		endpoint: endpoint,
	}
}

// Get implements HTTPGetter interface by sending GET request using configured client.
func (c *ClientMock) Get(urlPath string) (*http.Response, error) {
	// Notice that this is the client to interact with kubelet. In case of CAdvisor the prometheus.Do is used

	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := http.NewRequest(http.MethodGet, e.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request to: %s. Got error: %s ", e.String(), err)
	}

	return c.doer.Do(r)
}

// MetricFamiliesGetter returns a function that obtains metric families from a list of prometheus queries.
func (c *ClientMock) MetricFamiliesGetter(url string) prometheus.MetricsFamiliesGetter {
	return func(queries []prometheus.Query) ([]prometheus.MetricFamily, error) {
		mFamily, err := prometheus.GetFilteredMetricFamilies(c.doer, url, queries)
		if err != nil {
			return nil, fmt.Errorf("getting filtered metric families: %w", err)
		}

		return mFamily, nil
	}
}
