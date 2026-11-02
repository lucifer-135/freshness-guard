package webui_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/webui"
)

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           {Data: []byte(`<!doctype html><div id="root"></div>`)},
		"assets/app-abc123.js": {Data: []byte("console.log(1)")},
	}
}

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	webui.Handler(testFS()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestServesIndexAtPrefix(t *testing.T) {
	rec := get(t, "/freshness/ui/")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `<div id="root">`)
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
}

func TestServesHashedAssetsAsImmutable(t *testing.T) {
	rec := get(t, "/freshness/ui/assets/app-abc123.js")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript"), rec.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "console.log(1)", rec.Body.String())
}

func TestClientRoutesFallBackToIndex(t *testing.T) {
	rec := get(t, "/freshness/ui/reports/abc")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `<div id="root">`)
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
}

func TestMissingAssetIsNotFound(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, get(t, "/freshness/ui/assets/missing.js").Code)
}

func TestSetsSecurityHeaders(t *testing.T) {
	for _, path := range []string{"/freshness/ui/", "/freshness/ui/assets/app-abc123.js", "/freshness/ui/assets/missing.js"} {
		rec := get(t, path)
		assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"), path)
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), path)
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"), path)
	}
}

func TestStaticHasIndex(t *testing.T) {
	data, err := fs.ReadFile(webui.Static(), "index.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "<html")
}
