package cachepurge_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/cachepurge"
)

func TestClearSendsSetupToken(t *testing.T) {
	var method, path, token string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, token = r.Method, r.URL.EscapedPath(), r.Header.Get("X-Bifrost-Setup-Token")
		_, _ = w.Write([]byte(`{"message":"Cache cleared successfully"}`))
	}))
	defer srv.Close()

	require.NoError(t, (&cachepurge.Client{BaseURL: srv.URL, SetupToken: "tok"}).Clear(context.Background(), "abc/1"))
	assert.Equal(t, http.MethodDelete, method)
	assert.Equal(t, "/api/cache/clear/abc%2F1", path)
	assert.Equal(t, "tok", token)

	require.NoError(t, (&cachepurge.Client{BaseURL: srv.URL}).Clear(context.Background(), "abc"))
	assert.Empty(t, token, "no header without a token")
}

func TestClearReportsHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"dashboard auth is not configured and no setup token is set"}`))
	}))
	defer srv.Close()

	err := (&cachepurge.Client{BaseURL: srv.URL}).Clear(context.Background(), "abc")
	var clearErr *cachepurge.ClearError
	require.ErrorAs(t, err, &clearErr)
	assert.Equal(t, http.StatusForbidden, clearErr.Status)
	assert.Contains(t, clearErr.Body, "no setup token")
}
