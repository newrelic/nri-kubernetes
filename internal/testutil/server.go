package testutil

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
)

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
	return s.Server.URL + path.Join("/controlplane", component, "metrics")
}

func newServer(version Version) (*Server, error) {
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
