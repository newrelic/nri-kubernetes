package sink_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/src/integration/sink"
)

func testServer() (*httptest.Server, error) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		rw.WriteHeader(http.StatusOK)
	}))

	serverCert, err := tls.LoadX509KeyPair("testdata/server+3.pem", "testdata/server+3-key.pem")
	if err != nil {
		return nil, fmt.Errorf("loading server certificate: %w", err)
	}

	caCert, err := os.ReadFile("testdata/rootCA.pem")
	if err != nil {
		return nil, fmt.Errorf("loading CA certificate: %w", err)
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
	}

	server.TLS = &tlsConfig
	server.StartTLS()

	return server, nil
}

func TestTlsClient(t *testing.T) {
	server, err := testServer()
	if err != nil {
		t.Fatalf("Cannot create test server: %v", err)
	}

	defer server.Close()

	conf := config.TLSConfig{
		Enabled:  true,
		CertPath: "testdata/client-client.pem",
		KeyPath:  "testdata/client-client-key.pem",
		CAPath:   "testdata/rootCA.pem",
	}

	client, err := sink.NewTLSClient(conf)
	if err != nil {
		t.Fatalf("Error creating TLS client: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("TLS client failed to GET /: %v", err)
	}

	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Server responded with %d", resp.StatusCode)
	}
}
