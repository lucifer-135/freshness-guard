package api_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/api"
)

func TestHealthReturnsOK(t *testing.T) {
	srv := api.NewServer("127.0.0.1:0")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/freshness/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok","plugin":"freshness-guard"}`, rec.Body.String())
}

func TestUnknownRouteReturns404(t *testing.T) {
	srv := api.NewServer("127.0.0.1:0")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/freshness/unknown", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStartFailsWhenPortIsBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	assert.Error(t, api.NewServer(l.Addr().String()).Start())
}

func TestStartAndShutdown(t *testing.T) {
	srv := api.NewServer("127.0.0.1:0")
	require.NoError(t, srv.Start())
	assert.NoError(t, srv.Shutdown(context.Background()))
}
