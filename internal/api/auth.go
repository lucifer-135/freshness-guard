package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// requireToken protects a route with the configured bearer token. With an
// empty token the route is disabled, so the API is never open by accident.
func requireToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				writeError(w, http.StatusServiceUnavailable, "The plugin API is disabled because api.token is not configured.")
				return
			}
			given, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeError(w, http.StatusUnauthorized, "A valid bearer token is required.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
