// Tests for tripperWithBearerTokenAndRefresh — covers the kubelet TLS verification matrix.
package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTripperWithBearerTokenAndRefresh_TLSMatrix(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	tokenFile := writeTempFile(t, "kubelet-token", []byte("dummy-token"))
	serverCAPath := writeServerCAPEM(t, server)
	unrelatedCAPath := writeUnrelatedCAPEM(t)
	junkCAPath := writeTempFile(t, "junk-ca-*.pem", []byte("not a pem"))

	tests := []struct {
		name             string
		caBundlePath     string
		wantConstructErr error
		wantRequestOK    bool
		wantRequestErr   string
	}{
		{
			name:          "back-compat: empty path skips verification",
			caBundlePath:  "",
			wantRequestOK: true,
		},
		{
			name:          "valid CA matches server cert",
			caBundlePath:  serverCAPath,
			wantRequestOK: true,
		},
		{
			name:           "valid CA does not match server cert",
			caBundlePath:   unrelatedCAPath,
			wantRequestErr: "certificate signed by unknown authority",
		},
		{
			name:             "missing file returns os.ErrNotExist",
			caBundlePath:     filepath.Join(t.TempDir(), "does-not-exist.pem"),
			wantConstructErr: os.ErrNotExist,
		},
		{
			name:             "junk file returns errCABundleAppend",
			caBundlePath:     junkCAPath,
			wantConstructErr: errCABundleAppend,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tripper, err := tripperWithBearerTokenAndRefresh(tokenFile, tt.caBundlePath)

			if tt.wantConstructErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantConstructErr), "expected error to wrap %v, got %v", tt.wantConstructErr, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tripper)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			require.NoError(t, err)

			resp, err := tripper.RoundTrip(req)
			if tt.wantRequestOK {
				require.NoError(t, err)
				_ = resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				return
			}
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.wantRequestErr), "expected error containing %q, got %v", tt.wantRequestErr, err)
		})
	}
}

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func writeServerCAPEM(t *testing.T, server *httptest.Server) string {
	t.Helper()
	cert := server.Certificate()
	require.NotNil(t, cert, "httptest server must expose a certificate")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	return writeTempFile(t, "server-ca.pem", pemBytes)
}

func writeUnrelatedCAPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "unrelated-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return writeTempFile(t, "unrelated-ca.pem", pemBytes)
}

// TestBuildKubeletTLSConfig directly exercises the helper that builds the *tls.Config.
// Complements the end-to-end matrix above by asserting on the returned config struct
// rather than observed network behavior.
func TestBuildKubeletTLSConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty path: skip verification with TLS 1.2 minimum", func(t *testing.T) {
		t.Parallel()
		cfg, err := buildKubeletTLSConfig("")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.True(t, cfg.InsecureSkipVerify, "empty caBundlePath must set InsecureSkipVerify true (back-compat)")
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion, "MinVersion must be pinned to TLS 1.2")
		assert.Nil(t, cfg.RootCAs, "RootCAs must be nil when verification is skipped")
	})

	t.Run("valid CA path: verify with populated RootCAs and TLS 1.2 minimum", func(t *testing.T) {
		t.Parallel()
		caPath := writeUnrelatedCAPEM(t) // contents are valid PEM; identity does not matter for this assertion

		cfg, err := buildKubeletTLSConfig(caPath)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.False(t, cfg.InsecureSkipVerify, "non-empty caBundlePath must enable verification")
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion, "MinVersion must be pinned to TLS 1.2")
		require.NotNil(t, cfg.RootCAs, "RootCAs must be populated when caBundlePath is set")
		// crypto/x509.CertPool.Subjects is deprecated but remains a portable way to assert non-empty pool contents.
		assert.NotEmpty(t, cfg.RootCAs.Subjects(), "loaded RootCAs pool must contain at least one cert") //nolint:staticcheck // SA1019: testing-only pool inspection
	})

	t.Run("missing file: returns wrapped os.ErrNotExist", func(t *testing.T) {
		t.Parallel()
		cfg, err := buildKubeletTLSConfig(filepath.Join(t.TempDir(), "nope.pem"))
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.True(t, errors.Is(err, os.ErrNotExist), "expected os.ErrNotExist wrap, got %v", err)
	})

	t.Run("junk file: returns errCABundleAppend", func(t *testing.T) {
		t.Parallel()
		junk := writeTempFile(t, "junk-ca.pem", []byte("definitely not pem"))
		cfg, err := buildKubeletTLSConfig(junk)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.True(t, errors.Is(err, errCABundleAppend), "expected errCABundleAppend, got %v", err)
	})
}
