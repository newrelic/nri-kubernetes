package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
	"k8s.io/client-go/transport"
)

// ksm implements Client interface
type ksm struct {
	httpClient *http.Client
	endpoint   url.URL
	nodeIP     string
	logger     log.Logger
}

func newKSMClient(timeout time.Duration, nodeIP string, endpoint url.URL, logger log.Logger, k8s client.Kubernetes) *ksm {
	bearer := k8s.Config().BearerToken
	rt := newBearerRoundTripper(bearer)

	return &ksm{
		nodeIP:   nodeIP,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: rt,
		},
		logger: logger,
	}
}

func newBearerRoundTripper(bearer string) http.RoundTripper {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return transport.NewBearerAuthRoundTripper(bearer, baseTransport)
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
