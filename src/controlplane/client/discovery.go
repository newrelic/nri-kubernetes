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

	"github.com/newrelic/nri-kubernetes/v2/src/client"
	"github.com/newrelic/nri-kubernetes/v2/src/prometheus"
)

type invalidTLSConfig struct {
	message string
}

func (i invalidTLSConfig) Error() string {
	return i.message
}

type AuthenticationMethod string

const (
	None           AuthenticationMethod = "None (http)"
	MTLS           AuthenticationMethod = "Mutual TLS"
	ServiceAccount AuthenticationMethod = "Service account (Bearer token)"
)

// ControlPlaneComponentClient implements Client interface.
type ControlPlaneComponentClient struct {
	authenticationMethod AuthenticationMethod
	httpClient           *http.Client
	tlsSecretName        string
	tlsSecretNamespace   string
	logger               log.Logger
	k8sClient            kubernetes.Interface
	endpoint             url.URL
	secureEndpoint       url.URL
	nodeIP               string
	PodName              string
	InsecureFallback     bool
}

func New(
	authenticationMethod AuthenticationMethod,
	tlsSecretName string,
	tlsSecretNamespace string,
	logger log.Logger,
	k8sClient kubernetes.Interface,
	endpoint url.URL,
	secureEndpoint url.URL,
	nodeIP string,
	podName string,
	insecureFallback bool,
	timeout time.Duration,
) client.HTTPClient {
	return &ControlPlaneComponentClient{
		authenticationMethod: authenticationMethod,
		httpClient:           &http.Client{Timeout: timeout},
		tlsSecretName:        tlsSecretName,
		tlsSecretNamespace:   tlsSecretNamespace,
		logger:               logger,
		k8sClient:            k8sClient,
		endpoint:             endpoint,
		secureEndpoint:       secureEndpoint,
		nodeIP:               nodeIP,
		PodName:              podName,
		InsecureFallback:     insecureFallback,
	}
}

// Get implements HTTPGetter interface by selecting proper authentication strategy for request
// based on client configuration.
//
// If secure request fails and insecure fallback is configured, request will be attempted over HTTP.
func (c *ControlPlaneComponentClient) Get(urlPath string) (*http.Response, error) {
	// Use the secure endpoint by default. If this component doesn't support it yet, fallback to the insecure one.
	e := c.secureEndpoint
	usingSecureEndpoint := true
	if e.String() == "" {
		e = c.endpoint
		usingSecureEndpoint = false
	}

	r, err := c.buildPrometheusRequest(e, urlPath)
	if err != nil {
		return nil, err
	}

	if err = c.configureAuthentication(); err != nil {
		return nil, errors.Wrapf(err, "could not configure request for authentication method %s", c.authenticationMethod)
	}

	c.logger.Debugf("Calling endpoint: %s, authentication method: %s", r.URL.String(), string(c.authenticationMethod))

	resp, err := c.httpClient.Do(r)

	// If there is an error, we're using the secure endpoint and insecure fallback is on, we retry using the insecure
	// endpoint.
	if err != nil && usingSecureEndpoint && c.InsecureFallback {
		c.logger.Debugf("Error when calling secure endpoint: %s", err.Error())
		c.logger.Debugf("Falling back to insecure endpoint")
		e = c.endpoint
		r, err := c.buildPrometheusRequest(e, urlPath)
		if err != nil {
			return nil, err
		}
		return c.httpClient.Do(r)
	}

	return resp, err
}

func (c *ControlPlaneComponentClient) buildPrometheusRequest(e url.URL, urlPath string) (*http.Request, error) {
	e.Path = path.Join(e.Path, urlPath)
	r, err := prometheus.NewRequest(e.String())
	if err != nil {
		return nil, fmt.Errorf("Error creating request to: %s. Got error: %v ", e.String(), err)
	}
	return r, err
}

func (c *ControlPlaneComponentClient) configureAuthentication() error {
	switch c.authenticationMethod {
	case MTLS:
		tlsConfig, err := c.getTLSConfigFromSecret()
		if err != nil {
			return errors.Wrap(err, "could not load TLS configuration")
		}

		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	case ServiceAccount:
		config, err := rest.InClusterConfig()
		if err != nil {
			return errors.Wrapf(err, "could not create in cluster Kubernetes configuration to query pod: %s", c.PodName)
		}

		// Here we're using the default http.Transport configuration, but with a modified TLS config.
		// For some reason the DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		// Use the default kubernetes Bearer token authentication RoundTripper
		c.httpClient.Transport = transport.NewBearerAuthRoundTripper(config.BearerToken, t)
	default:
		c.httpClient.Transport = http.DefaultTransport
	}

	return nil
}

func (c *ControlPlaneComponentClient) getTLSConfigFromSecret() (*tls.Config, error) {
	namespace := c.tlsSecretNamespace
	if namespace == "" {
		c.logger.Debugf("TLS Secret name configured, but not TLS Secret namespace. Defaulting to `default` namespace.")
		namespace = "default"
	}

	secret, err := c.k8sClient.CoreV1().Secrets(namespace).Get(context.Background(), c.tlsSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not find secret %s containing TLS configuration", c.tlsSecretName)
	}

	var cert, key, cacert []byte

	var ok bool
	if cert, ok = secret.Data["cert"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS certificate in `cert` field in secret %s", c.tlsSecretName),
		}
	}

	if key, ok = secret.Data["key"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS key in `key` field in secret %s", c.tlsSecretName),
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

func (c *ControlPlaneComponentClient) NodeIP() string {
	return c.nodeIP
}
