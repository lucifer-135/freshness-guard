package api

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.New(slog.NewTextHandler(&buf, nil)))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { s.serve(l); close(done) }()
	require.NoError(t, l.Close())
	<-done
	assert.Contains(t, buf.String(), "stopped unexpectedly")
}

func TestShutdownIsNotLogged(t *testing.T) {
	var buf bytes.Buffer
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.New(slog.NewTextHandler(&buf, nil)))
	require.NoError(t, s.Start())
	require.NoError(t, s.Shutdown(context.Background()))
	assert.NotContains(t, buf.String(), "stopped unexpectedly")
}

func TestServerTimeouts(t *testing.T) {
	s := NewServer("127.0.0.1:0", NewHandler(Deps{}), slog.Default())
	assert.Equal(t, 30*time.Second, s.http.ReadTimeout)
	assert.Equal(t, 120*time.Second, s.http.IdleTimeout)
	assert.Equal(t, time.Duration(0), s.http.WriteTimeout, "event streams must not be cut off")
}
