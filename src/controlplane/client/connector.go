package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v2/src/client"
)

const (
	DefaultTimout      = 5000 * time.Millisecond
	defaultMetricsPath = "/metrics"
	mTLSAuth           = "mTLS"
	bearerAuth         = "bearer"
)

// Connector provides an interface to retrieve []connParams to connect to a Control Plane instance.
type Connector interface {
	Connect() (*connParams, error)
}

type defaultConnector struct {
	// TODO: Use a non-sdk logger
	logger          log.Logger
	kc              kubernetes.Interface
	inClusterConfig *rest.Config
	endpoints       []config.Endpoint
}

// DefaultConnector returns a defaultConnector that probes all endpoints in the list and return the first responding status OK.
func DefaultConnector(endpoints []config.Endpoint, kc kubernetes.Interface, inClusterConfig *rest.Config, logger log.Logger) (Connector, error) {
	if inClusterConfig == nil {
		return nil, fmt.Errorf("inClusterConfig cannot be nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if kc == nil {
		return nil, fmt.Errorf("kubernetes interface cannot be nil")
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("endpoints cannot be empty")
	}

	if err := validateEndpointConfig(endpoints); err != nil {
		return nil, fmt.Errorf("validating endpoints config: %w", err)
	}

	return &defaultConnector{
		logger:          logger,
		inClusterConfig: inClusterConfig,
		kc:              kc,
		endpoints:       endpoints,
	}, nil
}

// Connect iterates over the endpoints list probing each endpoint with a HEAD request
// and returns the connection parameters of the first endpoint that respond Status OK.
func (dp *defaultConnector) Connect() (*connParams, error) {
	for _, e := range dp.endpoints {
		dp.logger.Debugf("Configuring endpoint %q for probing", e.URL)

		u, err := url.Parse(e.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing endpoint url %q: %w", e.URL, err)
		}

		if u.Path == "" || u.Path == "/" {
			dp.logger.Debugf("Autodiscover endpoint %q does not contain path, adding default %q", e.URL, defaultMetricsPath)
			u.Path = defaultMetricsPath
		}

		httpClient, err := dp.newHTTPClient(e)
		if err != nil {
			return nil, fmt.Errorf("creating HTTP client for endpoint %q: %w", e.URL, err)
		}

		if err := dp.probeEndpoint(u.String(), httpClient); err != nil {
			dp.logger.Debugf("Endpoint %q probe failed, skipping: %v", e.URL, err)
			continue
		}

		dp.logger.Debugf("Endpoint %q probed successfully", e.URL)

		return &connParams{url: *u, client: httpClient}, nil
	}

	return nil, fmt.Errorf("all endpoints in the list failed to response")
}

func (dp *defaultConnector) probeEndpoint(url string, client *http.Client) error {
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("http HEAD request failed: %w", err)
	}

	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status: %v", resp.Status)
	}

	return nil
}

func (dp *defaultConnector) newHTTPClient(endpoint config.Endpoint) (*http.Client, error) {
	client := &http.Client{Timeout: DefaultTimout}

	// Here we're using the default http.Transport configuration, but with a modified TLS config.
	// For some reason the DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: endpoint.InsecureSkipVerify}
	client.Transport = t

	if err := dp.configureAuthentication(client, endpoint); err != nil {
		return nil, fmt.Errorf("configuring auth: %w", err)
	}

	return client, nil
}

func (dp *defaultConnector) configureAuthentication(httpClient *http.Client, endpoint config.Endpoint) error {
	if endpoint.Auth == nil {
		dp.logger.Debugf("No authentication configured for %q, connection will be attempted anonymously", endpoint.URL)

		return nil
	}

	if strings.EqualFold(endpoint.Auth.Type, bearerAuth) {
		dp.logger.Debugf("Using kubernetes token to authenticate request to %q", endpoint.URL)

		httpClient.Transport = transport.NewBearerAuthRoundTripper(dp.inClusterConfig.BearerToken, httpClient.Transport)

		return nil
	}

	if strings.EqualFold(endpoint.Auth.Type, mTLSAuth) {
		dp.logger.Debugf("Using mTLS to authenticate request to %q", endpoint.URL)

		tlsConfig, err := dp.getTLSConfigFromSecret(endpoint.Auth.MTLS)
		if err != nil {
			return fmt.Errorf("could not load TLS configuration: %w", err)
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}

		return nil
	}

	return fmt.Errorf("unknown authorization type %q", endpoint.Auth.Type)
}

func (dp *defaultConnector) getTLSConfigFromSecret(mTLSConfig *config.MTLS) (*tls.Config, error) {
	if mTLSConfig == nil {
		return nil, fmt.Errorf("mTLS config cannot be nil")
	}

	if mTLSConfig.TLSSecretName == "" {
		return nil, fmt.Errorf("mTLS secret name cannot be empty")
	}

	namespace := mTLSConfig.TLSSecretNamespace
	secretName := mTLSConfig.TLSSecretName
	if namespace == "" {
		dp.logger.Debugf("TLS Secret name configured, but not TLS Secret namespace. Defaulting to `default` namespace.")
		namespace = "default"
	}

	secret, err := dp.kc.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not find secret %q containing TLS configuration: %w", secretName, err)
	}

	var cert, key, cacert []byte

	var ok bool
	if cert, ok = secret.Data["cert"]; !ok {
		return nil, fmt.Errorf("could not find TLS certificate in `cert` field in secret %q", secretName)
	}

	if key, ok = secret.Data["key"]; !ok {
		return nil, fmt.Errorf("could not find TLS key in `key` field in secret %q", secretName)
	}

	cacert, hasCACert := secret.Data["cacert"]
	insecureSkipVerifyRaw, hasInsecureSkipVerify := secret.Data["insecureSkipVerify"]

	if !hasCACert && !hasInsecureSkipVerify {
		return nil, fmt.Errorf("both cacert and insecureSkipVerify are not set. One of them need to be set to be able to call ETCD metrics")
	}

	// insecureSkipVerify is set to false by default, and can be overridden with the insecureSkipVerify field
	insecureSkipVerify := false
	if hasInsecureSkipVerify {
		insecureSkipVerify = strings.ToLower(string(insecureSkipVerifyRaw)) == "true"
	}

	tlsConfig, err := parseTLSConfig(cert, key, cacert, insecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("parsing TLS config: %w", err)
	}

	return tlsConfig, nil
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

func validateEndpointConfig(endpoints []config.Endpoint) error {
	for _, e := range endpoints {
		if _, err := url.Parse(e.URL); err != nil {
			return fmt.Errorf("parsing endpoint url %q: %w", e.URL, err)
		}

		if err := validateAuth(e.Auth); err != nil {
			return fmt.Errorf("validating auth for endpoint url %q: %w", e.URL, err)
		}
	}

	return nil
}

func validateAuth(auth *config.Auth) error {
	if auth == nil {
		return nil
	}

	switch {
	case strings.EqualFold(auth.Type, bearerAuth):
		break
	case strings.EqualFold(auth.Type, mTLSAuth):
		return validateMTLS(auth.MTLS)
	default:
		return fmt.Errorf("authorization type not supported: %q", auth.Type)
	}

	return nil
}

func validateMTLS(mTLS *config.MTLS) error {
	if mTLS == nil {
		return fmt.Errorf("mTLS config must exist")
	}

	if mTLS.TLSSecretName == "" {
		return fmt.Errorf("TLSSecretName cannot be empty")
	}

	return nil
}

type connParams struct {
	url    url.URL
	client client.HTTPDoer
}
