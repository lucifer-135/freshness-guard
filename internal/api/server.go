// Package api serves the plugin's own HTTP API on a separate port, because
// Bifrost does not route unknown paths through plugin hooks.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server is the plugin's HTTP API server.
type Server struct {
	addr   string
	http   *http.Server
	logger *slog.Logger
}

// NewServer creates a server that will serve handler on addr.
func NewServer(addr string, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		addr: addr,
		http: &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second,
			// No WriteTimeout: event streams stay open for as long as the client listens.
			ReadTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second},
		logger: logger,
	}
}

// Start binds the listener synchronously, so a busy port is reported at once,
// and then serves requests in a goroutine.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	go s.serve(listener)
	return nil
}

func (s *Server) serve(listener net.Listener) {
	if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("The freshness-guard API server stopped unexpectedly.", "error", err)
	}
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","plugin":"freshness-guard"}`))
}
