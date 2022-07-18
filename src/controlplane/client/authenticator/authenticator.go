package authenticator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

const (
	mTLSAuth   = "mTLS"
	bearerAuth = "bearer"
)

// Authenticator provides an interface to generate a authorized round tripper.
type Authenticator interface {
	// AuthenticatedTransport returns a RoundTripper with the required configuration
	// to connect to the endpoint.
	AuthenticatedTransport(endpoint config.Endpoint) (http.RoundTripper, error)
}

type Config struct {
	SecretListerer  discovery.SecretListerer
	InClusterConfig *rest.Config
}

type OptionFunc func(kca *K8sClientAuthenticator) error

// WithLogger returns an OptionFunc to change the logger from the default noop logger.
func WithLogger(logger *log.Logger) OptionFunc {
	return func(kca *K8sClientAuthenticator) error {
		kca.logger = logger
		return nil
	}
}

type K8sClientAuthenticator struct {
	Config
	logger *log.Logger
}

// New returns an K8sClientAuthenticator that supports plain, bearer token and mTLS.
func New(config Config, opts ...OptionFunc) (*K8sClientAuthenticator, error) {
	kca := &K8sClientAuthenticator{
		logger: logutil.Discard,
		Config: config,
	}

	for i, opt := range opts {
		if err := opt(kca); err != nil {
			return nil, fmt.Errorf("applying option #%d: %w", i, err)
		}
	}

	return kca, nil
}

// AuthenticatedTransport returns a round tripper according to the endpoint config.
// For mTLS configuration it fetches the certificates from the secret.
func (a K8sClientAuthenticator) AuthenticatedTransport(endpoint config.Endpoint) (http.RoundTripper, error) {
	transportConfig := &transport.Config{
		TLS: transport.TLSConfig{
			Insecure: endpoint.InsecureSkipVerify,
		},
	}

	switch {
	case endpoint.Auth == nil:
		a.logger.Debugf("No authentication configured for %q, connection will be attempted anonymously", endpoint.URL)

	case strings.EqualFold(endpoint.Auth.Type, bearerAuth):
		a.logger.Debugf("Using kubernetes token to authenticate request to %q", endpoint.URL)

		transportConfig.BearerTokenFile = a.InClusterConfig.BearerTokenFile

	case strings.EqualFold(endpoint.Auth.Type, mTLSAuth) && endpoint.Auth.MTLS != nil:
		a.logger.Debugf("Using mTLS to authenticate request to %q", endpoint.URL)

		certs, err := a.getTLSCertificatesFromSecret(endpoint.Auth.MTLS)
		if err != nil {
			return nil, fmt.Errorf("could not load TLS configuration for endpoint %q: %w", endpoint.URL, err)
		}

		if certs.ca == nil && !endpoint.InsecureSkipVerify {
			return nil, fmt.Errorf("insecureSkipVerify is false and CA cert is missing from secret %q", endpoint.URL)
		}

		transportConfig.TLS.CertData = certs.cert
		transportConfig.TLS.KeyData = certs.key
		transportConfig.TLS.CAData = certs.ca

	default:
		return nil, fmt.Errorf("unknown authorization type %q", endpoint.Auth.Type)
	}

	rt, err := transport.New(transportConfig)
	if err != nil {
		return nil, fmt.Errorf("creating the round tripper: %w", err)
	}

	return rt, nil
}

// certificatesData contains bytes of the PEM-encoded certificates.
type certificatesData struct {
	cert []byte
	key  []byte
	ca   []byte
}

// certificateSecretKeys contains the name of the keys inside the secret where the certificate, private key, and CA
// certificates are stored.
// Earlier versions of the integration allowed to define a secret of type corev1.SecretTypeOpaque, with the required
// certs and keys stored in keys named as the constants below.
// New versions can also consume a secret of type corev1.SecretTypeTLS, using standard names for certificate and key.
type certificateSecretKeys struct {
	cert string
	key  string
	ca   string
}

// opaqueCertKey is the key for the secret data where the PEM-encoded certificate is located in Opaque secrets.
const opaqueCertKey = "cert"

// opaqueKeyKey is the key for the secret data where the PEM-encoded private key is located  Opaque secrets.
const opaqueKeyKey = "key"

// opaqueCacertKey is the key for the secret data where the PEM-encoded CA certificate key is located Opaque secrets .
const opaqueCacertKey = "cacert"

// tlsCertCaName is the key name for the CA Certificate inside the secret data.
// Unlike v1.TLSCertKey and v1.TLSPrivateKeyKey, the key for the CA certificate is not standard. Here we mirror the
// one Cert manager is using:
// https://github.com/jetstack/cert-manager/blob/83d722f267c4dbc69d2e1c274bfb2eac5a49ca9c/internal/apis/meta/types.go#L74
const tlsCacertKey = "ca.crt"

// getTLSCertificatesFromSecret fetches the certificates from the secrets using the secret lister.
func (a K8sClientAuthenticator) getTLSCertificatesFromSecret(mTLSConfig *config.MTLS) (*certificatesData, error) {
	if mTLSConfig.TLSSecretName == "" {
		return nil, fmt.Errorf("mTLS secret name cannot be empty")
	}

	if mTLSConfig.TLSSecretNamespace == "" {
		return nil, fmt.Errorf("mTLS secret namespace cannot be empty")
	}

	secretLister, ok := a.SecretListerer.Lister(mTLSConfig.TLSSecretNamespace)
	if !ok {
		return nil, fmt.Errorf("could not find secret lister for namespace %q", mTLSConfig.TLSSecretNamespace)
	}

	a.logger.Debugf("Getting TLS certs from secret %q on namespace %q", mTLSConfig.TLSSecretName, mTLSConfig.TLSSecretNamespace)

	secret, err := secretLister.Get(mTLSConfig.TLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("could not find secret %q containing TLS configuration: %w", mTLSConfig.TLSSecretName, err)
	}

	keynames := certificateSecretKeys{
		cert: opaqueCertKey,
		key:  opaqueKeyKey,
		ca:   opaqueCacertKey,
	}

	if secret.Type == corev1.SecretTypeTLS {
		a.logger.Debugf("Secret %q has type %q, using standard key names", secret.Name, secret.Type)

		keynames = certificateSecretKeys{
			cert: corev1.TLSCertKey,
			key:  corev1.TLSPrivateKeyKey,
			ca:   tlsCacertKey,
		}
	}

	certdata := &certificatesData{}

	if certdata.cert, ok = secret.Data[keynames.cert]; !ok {
		return nil, fmt.Errorf("could not find TLS certificate in %q field in secret %q", keynames.cert, secret.Name)
	}

	if certdata.key, ok = secret.Data[keynames.key]; !ok {
		return nil, fmt.Errorf("could not find TLS key in %q field in secret %q", keynames.key, secret.Name)
	}

	if certdata.ca, ok = secret.Data[keynames.ca]; !ok {
		a.logger.Debugf("CA certificate not found in %q field in secret %q. CA will not be validated.", keynames.ca, secret.Name)
	}

	return certdata, nil
}
