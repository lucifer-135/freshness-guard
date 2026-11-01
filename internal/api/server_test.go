package api_test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
)

func newServer(addr string) *api.Server {
	return api.NewServer(addr, api.NewHandler(api.Deps{}), slog.Default())
}

func TestHealthReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	api.NewHandler(api.Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/freshness/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok","plugin":"freshness-guard"}`, rec.Body.String())
}

func TestUnknownRouteReturns404(t *testing.T) {
	rec := httptest.NewRecorder()
	api.NewHandler(api.Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/freshness/unknown", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStartFailsWhenPortIsBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	assert.Error(t, newServer(l.Addr().String()).Start())
}

func TestStartAndShutdown(t *testing.T) {
	srv := newServer("127.0.0.1:0")
	require.NoError(t, srv.Start())
	assert.NoError(t, srv.Shutdown(context.Background()))
}
