package sink

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
)

var ErrCAAppend = errors.New("appending certs to pool")

func NewTLSClient(conf config.TLSConfig) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(conf.CertPath, conf.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading client certificates: %w", err)
	}

	caCert, err := os.ReadFile(conf.CAPath)
	if err != nil {
		return nil, fmt.Errorf("loading CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("%w from %q", ErrCAAppend, conf.CAPath)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caCertPool,
			},
		},
	}

	return client, nil
}
