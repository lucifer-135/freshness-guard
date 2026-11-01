// Package api serves the plugin's own HTTP API on a separate port, because
// Bifrost does not route unknown paths through plugin hooks.
package api

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Server is the plugin's HTTP API server.
type Server struct {
	addr string
	http *http.Server
}

// NewServer creates a server that will listen on addr.
func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /freshness/health", handleHealth)
	return &Server{
		addr: addr,
		http: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second},
	}
}

// Handler returns the server's request handler.
func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

// Start binds the listener synchronously, so a busy port is reported at once,
// and then serves requests in a goroutine.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	go func() { _ = s.http.Serve(listener) }()
	return nil
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","plugin":"freshness-guard"}`))
}
