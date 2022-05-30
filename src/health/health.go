// Package health implements a simple liveness probe.
package health

import (
	"io"
	"net/http"
	"sync"

	"github.com/newrelic/nri-kubernetes/v3/internal/logutil"
	"github.com/sirupsen/logrus"
)

const (
	ReasonUninitialized = "Application has not been initialized yet"
)

// Server is an HTTP server that can be used as a liveness probe.
type Server struct {
	lock    sync.RWMutex
	logger  *logrus.Logger
	healthy bool
	reason  string
}

type OptionFunc func(server *Server)

func WithLogger(logger *logrus.Logger) OptionFunc {
	return func(server *Server) {
		server.logger = logger
	}
}

// New creates a new health.Server. A newly created Server is marked as unhealthy with ReasonUninitialized as the reason.
func New(options ...OptionFunc) *Server {
	s := &Server{
		healthy: false,
		reason:  ReasonUninitialized,
		logger:  logutil.Discard,
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

// ListenAndServe sets the health.Server to listen in the supplied address, blocking until an error occurs.
func (s *Server) ListenAndServe(address string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)

	s.logger.Infof("Starting health server on %s", address)

	return http.ListenAndServe(address, mux)
}

// Healthy can be used to signal the Server the application is healthy. After Healthy has been called, the server will
// start returning http.StatusOK until Unhealthy is called.
func (s *Server) Healthy() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.logger.Debugf("Server marked as healthy")

	s.healthy = true
	s.reason = ""
}

// Unhealthy can be used to signal the Server the application is not healthy. If a reason is included, the server will
// echo it in the HTTP response body.
func (s *Server) Unhealthy(reason string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.logger.Debugf("Server marked as unhealthy due to %q", reason)

	s.healthy = false
	s.reason = reason
}

// health is an http.HandlerFunc that returns http.StatusOK if server is flagged as healthy,
// and http.StatusServiceUnavailable if it is not.
func (s *Server) health(rw http.ResponseWriter, _ *http.Request) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.healthy {
		s.logger.Debugf("Server healthy, returning 200")
		rw.WriteHeader(http.StatusOK)
		return
	}

	s.logger.Infof("Server unhealthy: %s", s.reason)
	rw.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(rw, s.reason)
}
