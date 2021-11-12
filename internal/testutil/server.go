package testutil

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
)

type Version string

// List of all the versions we have testdata for.
// When adding a new version:
// - REMEMBER TO ADD IT TO AllVersions() BELOW.
// - UPDATE LatestVersion() BELOW IF NEEDED

const (
	Testdata116 = "1_16"
	Testdata118 = "1_18"
)

// LatestVersion returns the latest version we have test data for.
func LatestVersion() Version {
	return Testdata118
}

// AllVersions returns a list of versions we have test data for.
func AllVersions() []Version {
	return []Version{
		Testdata116,
		Testdata118,
	}
}

//go:embed data
var testDataDir embed.FS

// Name of the root folder in embed.FS
const testDataRootDir = "data"

type Server struct {
	*httptest.Server
}

func (s *Server) KSMEndpoint() string {
	// We must add /metrics to the URL here as the KSM override endpoint must be a full URL
	return s.Server.URL + "/ksm/metrics"
}
func (s *Server) KubeletEndpoint() string {
	return s.Server.URL + "/kubelet"
}
func (s *Server) ControlPlaneEndpoint(component string) string {
	return s.Server.URL + "/controlplane/" + strings.ReplaceAll(component, "/", "")
}

func NewServer(version Version) (*Server, error) {
	subversion, err := fs.Sub(testDataDir, filepath.Join(testDataRootDir, string(version)))
	if err != nil {
		return nil, fmt.Errorf("opening dir for version %s: %w", version, err)
	}

	fileserver := http.FileServer(http.FS(subversion))
	testServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("server", "testutil fake http server")
		rw.Header().Set("testutil-data-version", string(version))

		fileserver.ServeHTTP(rw, r)
	}))

	return &Server{testServer}, nil
}
