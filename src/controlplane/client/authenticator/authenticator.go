package authenticator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	DefaultSecretNamespace = "default"
	mTLSAuth               = "mTLS"
	bearerAuth             = "bearer"
)

// Authenticator provides an interface to generate a authorized round tripper.
type Authenticator interface {
	// AuthenticatedTransport returns a RoundTripper with the required configuration
	// to connect to the endpoint.
	AuthenticatedTransport(endpoint config.Endpoint) (http.RoundTripper, error)
}

type Config struct {
	SecretListerByNamespace map[string]v1.SecretNamespaceLister
	InClusterConfig         *rest.Config
}

type OptionFunc func(kca *K8sClientAuthenticator) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger log.Logger) OptionFunc {
	return func(kca *K8sClientAuthenticator) error {
		kca.logger = logger
		return nil
	}
}

type K8sClientAuthenticator struct {
	Config
	logger log.Logger
}

// New returns an K8sClientAuthenticator that supports plain, bearer token and mTLS.
func New(config Config, opts ...OptionFunc) (*K8sClientAuthenticator, error) {
	kca := &K8sClientAuthenticator{
		logger: log.Discard,
		Config: config,
	}

	for i, opt := range opts {
		if err := opt(kca); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return kca, nil
}

// Authenticate retruns a round tripper according to the endpoint config.
// For mTLS configuration it fetches the certificates from the secret.
func (a K8sClientAuthenticator) AuthenticatedTransport(endpoint config.Endpoint) (http.RoundTripper, error) {
	transportConfig := &transport.Config{}
	tlsConfig := transport.TLSConfig{
		Insecure: endpoint.InsecureSkipVerify,
	}

	switch {
	case endpoint.Auth == nil:
		a.logger.Debugf("No authentication configured for %q, connection will be attempted anonymously", endpoint.URL)

	case strings.EqualFold(endpoint.Auth.Type, bearerAuth):
		a.logger.Debugf("Using kubernetes token to authenticate request to %q", endpoint.URL)

		transportConfig.BearerToken = a.InClusterConfig.BearerToken

	case strings.EqualFold(endpoint.Auth.Type, mTLSAuth) && endpoint.Auth.MTLS != nil:
		a.logger.Debugf("Using mTLS to authenticate request to %q", endpoint.URL)

		certs, err := a.getTLSCertificatesFromSecret(endpoint.Auth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("could not load TLS configuration: %w", err)
		}

		tlsConfig.CertData = certs.cert
		tlsConfig.KeyData = certs.key
		// CAData could be empty if insecureSkipVerify is true.
		tlsConfig.CAData = certs.ca

	default:
		return nil, fmt.Errorf("unknown authorization type %q", endpoint.Auth.Type)
	}

	transportConfig.TLS = tlsConfig

	rt, err := transport.New(transportConfig)
	if err != nil {
		return nil, fmt.Errorf("creating the round tripper: %w", err)
	}

	return rt, nil
}

// certificatesData contains bytes of the PEM-encoded certificates
type certificatesData struct {
	cert []byte
	key  []byte
	ca   []byte
}

// getTLSCertificatesFromSecret fetches the certificates from the secrets using the secret lister.
func (a K8sClientAuthenticator) getTLSCertificatesFromSecret(mTLSConfig *config.MTLS) (*certificatesData, error) {
	if mTLSConfig.TLSSecretName == "" {
		return nil, fmt.Errorf("mTLS secret name cannot be empty")
	}

	namespace := mTLSConfig.TLSSecretNamespace
	if namespace == "" {
		a.logger.Debugf("TLS Secret name configured, but not TLS Secret namespace. Defaulting to `default` namespace.")

		namespace = DefaultSecretNamespace
	}

	var secretLister v1.SecretNamespaceLister

	var ok bool

	if secretLister, ok = a.SecretListerByNamespace[namespace]; !ok {
		return nil, fmt.Errorf("could not find secret lister for namespace %q", namespace)
	}

	a.logger.Debugf("Getting TLS certs from secret %q on namespace %q", mTLSConfig.TLSSecretName, namespace)

	secret, err := secretLister.Get(mTLSConfig.TLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("could not find secret %q containing TLS configuration: %w", mTLSConfig.TLSSecretName, err)
	}

	var cert, key, cacert []byte

	if cert, ok = secret.Data["cert"]; !ok {
		return nil, fmt.Errorf("could not find TLS certificate in `cert` field in secret %q", mTLSConfig.TLSSecretName)
	}

	if key, ok = secret.Data["key"]; !ok {
		return nil, fmt.Errorf("could not find TLS key in `key` field in secret %q", mTLSConfig.TLSSecretName)
	}

	if cacert, ok = secret.Data["cacert"]; !ok {
		a.logger.Debugf("CA certificate is not present in secret %q on namespace %q", mTLSConfig.TLSSecretName, namespace)
	}

	return &certificatesData{
		cert: cert,
		key:  key,
		ca:   cacert,
	}, nil
}
