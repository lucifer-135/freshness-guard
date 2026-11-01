package api

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"testing"

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
