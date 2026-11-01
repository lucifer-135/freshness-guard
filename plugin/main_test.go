package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/app"
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

func initWith(t *testing.T, port int, dbPath string) {
	t.Helper()
	require.NoError(t, Init(map[string]any{
		"api":     map[string]any{"port": float64(port), "token": "t"},
		"db_path": dbPath,
	}))
}

func newCtx() *schemas.BifrostContext {
	return schemas.NewBifrostContext(context.Background(), time.Now().Add(time.Minute))
}

// A reload holds mu while it swaps apps; request hooks must not wait for it.
func TestHooksDoNotWaitForInit(t *testing.T) {
	mu.Lock()
	done := make(chan struct{})
	go func() {
		_ = HTTPTransportResponseHeadersHook(newCtx(), &schemas.HTTPRequest{}, &schemas.HTTPResponseMetadata{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("a request hook waited for Init")
	}
	mu.Unlock()
	<-done
}

// A reload frees the old API port at once but drains the old app's jobs in
// the background, so Init returns without waiting for them.
func TestReloadDrainsOldAppInBackground(t *testing.T) {
	release, drained := make(chan struct{}), make(chan struct{})
	orig := drainApp
	drainApp = func(a *app.App) { <-release; orig(a); close(drained) }
	defer func() { drainApp = orig }()

	oldPort, newPort, db := freePort(t), freePort(t), filepath.Join(t.TempDir(), "f.db")
	initWith(t, oldPort, db)
	start := time.Now()
	initWith(t, newPort, db)
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.True(t, healthOK(newPort), "the new app serves at once")
	assert.False(t, healthOK(oldPort), "the old port is free at once")

	close(release)
	<-drained
	require.NoError(t, Cleanup())
	require.NoError(t, Cleanup())
	assert.Nil(t, getApp())
}

func TestGetName(t *testing.T) {
	assert.Equal(t, "freshness-guard", GetName())
}

// Bifrost reloads a plugin by calling Init again on the same loaded .so before
// it retires the old instance, so Init and Cleanup must pair up.
func TestInitTwiceThenCleanupOnceKeepsServer(t *testing.T) {
	port, db := freePort(t), filepath.Join(t.TempDir(), "f.db")
	initWith(t, port, db)
	initWith(t, port, db)
	require.NoError(t, Cleanup())
	assert.True(t, healthOK(port), "health must still answer after one of two Cleanup calls")
	require.NoError(t, Cleanup())
	assert.False(t, healthOK(port), "health must stop after the last Cleanup")
}

func TestInitWithNewPortMovesServer(t *testing.T) {
	oldPort, newPort, db := freePort(t), freePort(t), filepath.Join(t.TempDir(), "f.db")
	initWith(t, oldPort, db)
	initWith(t, newPort, db)
	defer func() { _ = Cleanup(); _ = Cleanup() }()
	assert.True(t, healthOK(newPort), "health must answer on the new port")
	assert.False(t, healthOK(oldPort), "the old listener must be closed")
}

func TestInitSucceedsWhenAPIPortBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	initWith(t, l.Addr().(*net.TCPAddr).Port, filepath.Join(t.TempDir(), "f.db"))
	assert.NotNil(t, getApp(), "the pipeline keeps running without the API")
	assert.NoError(t, Cleanup())
}

func TestInitWithBadDatabaseStillSucceeds(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	initWith(t, freePort(t), filepath.Join(file, "f.db"))
	defer Cleanup()
	assert.Nil(t, getApp())
	md := &schemas.HTTPResponseMetadata{}
	assert.NoError(t, HTTPTransportResponseHeadersHook(newCtx(), &schemas.HTTPRequest{}, md))
	assert.Equal(t, "error", md.Header("X-Freshness-Status"))
}

func TestHooksSetSkippedWithoutAnswer(t *testing.T) {
	initWith(t, freePort(t), filepath.Join(t.TempDir(), "f.db"))
	defer Cleanup()
	ctx := newCtx()
	resp, err := HTTPTransportPreHook(ctx, &schemas.HTTPRequest{})
	assert.Nil(t, resp)
	assert.NoError(t, err)
	md := &schemas.HTTPResponseMetadata{}
	assert.NoError(t, HTTPTransportResponseHeadersHook(ctx, &schemas.HTTPRequest{}, md))
	assert.Equal(t, "skipped", md.Header("X-Freshness-Status"))
}

func TestLLMHooksPassThrough(t *testing.T) {
	initWith(t, freePort(t), filepath.Join(t.TempDir(), "f.db"))
	defer Cleanup()
	ctx := newCtx()
	req := &schemas.BifrostRequest{RequestType: schemas.ChatCompletionRequest}
	gotReq, short, err := PreLLMHook(ctx, req)
	assert.Same(t, req, gotReq)
	assert.Nil(t, short)
	assert.NoError(t, err)

	resp, bErr := &schemas.BifrostResponse{}, &schemas.BifrostError{}
	gotResp, gotErr, err := PostLLMHook(ctx, resp, bErr)
	assert.Same(t, resp, gotResp)
	assert.Same(t, bErr, gotErr)
	assert.NoError(t, err)
}

func TestResponseHeadersHookNilMetadata(t *testing.T) {
	ctx := schemas.NewBifrostContext(context.Background(), time.Now().Add(time.Minute))
	assert.NoError(t, HTTPTransportResponseHeadersHook(ctx, nil, nil))
}
