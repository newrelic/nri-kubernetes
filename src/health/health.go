// Package health implements a simple liveness probe.
package health

import (
	"io"
	"net/http"
	"sync"
)

const (
	ReasonUninitialized = "Application has not been initialized yet"
)

// Server is an HTTP server that can be used as a liveness probe.
type Server struct {
	lock sync.RWMutex

	healthy bool
	reason  string
}

// New creates a new health.Server. A newly created Server is marked as unhealthy with ReasonUninitialized as the reason.
func New() *Server {
	return &Server{
		healthy: false,
		reason:  ReasonUninitialized,
	}
}

// ListenAndServe sets the health.Server to listen in the supplied address, blocking until an error occurs.
func (s *Server) ListenAndServe(address string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)

	return http.ListenAndServe(address, mux)
}

// Healthy can be used to signal the Server the application is healthy. After Healthy has been called, the server will
// start returning http.StatusOK until Unhealthy is called.
func (s *Server) Healthy() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.healthy = true
	s.reason = ""
}

// Unhealthy can be used to signal the Server the application is not healthy. If a reason is included, the server will
// echo it in the HTTP response body.
func (s *Server) Unhealthy(reason string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.healthy = false
	s.reason = reason
}

// health is an http.HandlerFunc that returns http.StatusOK if server is flagged as healthy,
// and http.StatusServiceUnavailable if it is not.
func (s *Server) health(rw http.ResponseWriter, _ *http.Request) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.healthy {
		rw.WriteHeader(http.StatusOK)
		return
	}

	rw.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(rw, s.reason)
}
