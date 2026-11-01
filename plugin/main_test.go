package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "freshness-guard", GetName())
}

func TestInitSucceedsWhenAPIPortBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	assert.NoError(t, Init(map[string]any{"api": map[string]any{"port": float64(port)}}))
	assert.NoError(t, Cleanup())
}

func TestResponseHeadersHookSetsSkipped(t *testing.T) {
	require.NoError(t, Init(map[string]any{"api": map[string]any{"port": float64(freePort(t))}}))
	defer Cleanup()
	md := &schemas.HTTPResponseMetadata{}
	ctx := schemas.NewBifrostContext(context.Background(), time.Now().Add(time.Minute))
	assert.NoError(t, HTTPTransportResponseHeadersHook(ctx, &schemas.HTTPRequest{}, md))
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestResponseHeadersHookNilMetadata(t *testing.T) {
	ctx := schemas.NewBifrostContext(context.Background(), time.Now().Add(time.Minute))
	assert.NoError(t, HTTPTransportResponseHeadersHook(ctx, nil, nil))
}
