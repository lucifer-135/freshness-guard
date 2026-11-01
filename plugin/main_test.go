package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
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

func healthOK(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/freshness/health", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func initWithPort(t *testing.T, port int) {
	t.Helper()
	require.NoError(t, Init(map[string]any{"api": map[string]any{"port": float64(port)}}))
}

// Bifrost reloads a plugin by calling Init again on the same loaded .so before
// it retires the old instance, so Init and Cleanup must pair up.
func TestInitTwiceThenCleanupOnceKeepsServer(t *testing.T) {
	port := freePort(t)
	initWithPort(t, port)
	initWithPort(t, port)
	require.NoError(t, Cleanup())
	assert.True(t, healthOK(port), "health must still answer after one of two Cleanup calls")
	require.NoError(t, Cleanup())
	assert.False(t, healthOK(port), "health must stop after the last Cleanup")
}

func TestInitWithNewPortMovesServer(t *testing.T) {
	oldPort, newPort := freePort(t), freePort(t)
	initWithPort(t, oldPort)
	initWithPort(t, newPort)
	defer func() { _ = Cleanup(); _ = Cleanup() }()
	assert.True(t, healthOK(newPort), "health must answer on the new port")
	assert.False(t, healthOK(oldPort), "the old listener must be closed")
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
	mu.Lock()
	assert.Nil(t, server, "no server is kept when the port is busy")
	mu.Unlock()
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
