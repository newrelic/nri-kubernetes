package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type invalidTLSConfig struct {
	message string
}

func (i invalidTLSConfig) Error() string {
	return i.message
}

type Client struct {
	httpClient      *http.Client
	logger          log.Logger
	k8sClient       kubernetes.Interface
	InClusterConfig *rest.Config
	endpoint        *url.URL
	auth            *config.Auth
}

type Config struct {
	Logger          log.Logger
	K8sClient       kubernetes.Interface
	InClusterConfig *rest.Config
	EndpoinURL      string
	Auth            *config.Auth
}

func New(cfg Config) (client.HTTPClient, error) {
	if cfg.EndpoinURL == "" {
		return nil, fmt.Errorf("URL must not be empty")
	}

	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger must not be nil")
	}

	if cfg.InClusterConfig == nil {
		return nil, fmt.Errorf("InClusterConfig must not be nil")
	}

	if cfg.Auth != nil && cfg.K8sClient == nil {
		return nil, fmt.Errorf("k8s client must not be nil if Auth is set")
	}

	u, err := url.Parse(cfg.EndpoinURL)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint url %s: %w", cfg.EndpoinURL, err)
	}

	c := &Client{
		httpClient:      &http.Client{Timeout: 500 * time.Millisecond},
		logger:          cfg.Logger,
		k8sClient:       cfg.K8sClient,
		InClusterConfig: cfg.InClusterConfig,
		auth:            cfg.Auth,
		endpoint:        u,
	}

	if err = c.configureAuthentication(); err != nil {
		return nil, fmt.Errorf("fail configuring auth method: %w", err)
	}

	return c, nil
}

// Get implements HTTPGetter interface by selecting proper authentication strategy for request
// based on client configuration.
//
// TODO If secure request fails and insecure fallback is configured, request will be attempted over HTTP.
func (c *Client) Get(urlPath string) (*http.Response, error) {
	endpoint := *c.endpoint
	endpoint.Path = path.Join(endpoint.Path, urlPath)

	req, err := prometheus.NewRequest(endpoint.String())
	if err != nil {
		return nil, fmt.Errorf("Error creating request to: %s. Got error: %v ", endpoint.String(), err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) configureAuthentication() error {
	switch {
	case c.auth != nil && c.auth.TLSSecretName != "":
		tlsConfig, err := c.getTLSConfigFromSecret()
		if err != nil {
			return errors.Wrap(err, "could not load TLS configuration")
		}

		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	case c.endpoint.Scheme == "https":
		// Here we're using the default http.Transport configuration, but with a modified TLS config.
		// For some reason the DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		// Use the default kubernetes Bearer token authentication RoundTripper
		c.httpClient.Transport = transport.NewBearerAuthRoundTripper(c.InClusterConfig.BearerToken, t)
	case c.endpoint.Scheme == "http":
		c.httpClient.Transport = http.DefaultTransport
	}

	return nil
}

func (c *Client) getTLSConfigFromSecret() (*tls.Config, error) {
	namespace := c.auth.TLSSecretNamespace
	secretName := c.auth.TLSSecretName
	if namespace == "" {
		c.logger.Debugf("TLS Secret name configured, but not TLS Secret namespace. Defaulting to `default` namespace.")
		namespace = "default"
	}

	secret, err := c.k8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not find secret %s containing TLS configuration", secretName)
	}

	var cert, key, cacert []byte

	var ok bool
	if cert, ok = secret.Data["cert"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS certificate in `cert` field in secret %s", secretName),
		}
	}

	if key, ok = secret.Data["key"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS key in `key` field in secret %s", secretName),
		}
	}

	cacert, hasCACert := secret.Data["cacert"]
	insecureSkipVerifyRaw, hasInsecureSkipVerify := secret.Data["insecureSkipVerify"]

	if !hasCACert && !hasInsecureSkipVerify {
		return nil, invalidTLSConfig{
			message: "both cacert and insecureSkipVerify are not set. One of them need to be set to be able to call ETCD metrics",
		}
	}

	// insecureSkipVerify is set to false by default, and can be overridden with the insecureSkipVerify field
	insecureSkipVerify := false
	if hasInsecureSkipVerify {
		insecureSkipVerify = strings.ToLower(string(insecureSkipVerifyRaw)) == "true"
	}

	return parseTLSConfig(cert, key, cacert, insecureSkipVerify)
}

func parseTLSConfig(certPEMBlock, keyPEMBlock, cacertPEMBlock []byte, insecureSkipVerify bool) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	clientCertPool := x509.NewCertPool()

	if len(cacertPEMBlock) > 0 {
		clientCertPool.AppendCertsFromPEM(cacertPEMBlock)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            clientCertPool,
		InsecureSkipVerify: insecureSkipVerify,
	}

	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
