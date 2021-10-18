package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

// ksm implements Client interface
type ksm struct {
	httpClient *http.Client
	endpoint   url.URL
	nodeIP     string
	logger     log.Logger
}

func newKSMClient(timeout time.Duration, nodeIP string, endpoint url.URL, logger log.Logger) *ksm {
	return &ksm{
		nodeIP:   nodeIP,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		logger: logger,
	}
}

func (c *ksm) NodeIP() string {
	return c.nodeIP
}

func (c *ksm) Do(method, urlPath string) (*http.Response, error) {
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := prometheus.NewRequest(method, e.String())
	if err != nil {
		return nil, fmt.Errorf("Error creating %s request to: %s. Got error: %s ", method, e.String(), err)
	}

	c.logger.Debugf("Calling kube-state-metrics endpoint: %s", r.URL.String())

	return c.httpClient.Do(r)
}
