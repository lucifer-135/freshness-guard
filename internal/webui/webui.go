// Package webui serves the web app's static build. The Dockerfile copies the
// build into static/, where it is embedded into the plugin; without a build,
// static/ holds a placeholder page.
package webui

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// Prefix is the URL path under which the web app is served.
const Prefix = "/freshness/ui/"

// The app loads nothing from other origins: fonts are bundled and every API
// call goes to the plugin API itself. Inline styles are allowed because the
// animation library sets style attributes.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

//go:embed all:static
var embedded embed.FS

// Static returns the embedded build.
func Static() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		// static/ is part of this package, so fs.Sub cannot fail.
		panic(err)
	}
	return sub
}

// Handler serves fsys under Prefix. An existing file is served as it is. A path
// without a file extension is a client-side route and gets index.html. A
// missing file with an extension is not found. Hashed files under assets/ are
// cached for good; everything else is revalidated on each load.
func Handler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")

		name := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(r.URL.Path, Prefix)), "/")
		if name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(fsys, name)
		if errors.Is(err, fs.ErrNotExist) && path.Ext(name) == "" {
			name = "index.html"
			data, err = fs.ReadFile(fsys, name)
		}
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "The web app could not be loaded.", http.StatusInternalServerError)
			return
		}

		if strings.HasPrefix(name, "assets/") {
			header.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			header.Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
