// Package hooks holds the logic behind the plugin's exported Bifrost hooks.
package hooks

import (
	"log/slog"
	"runtime/debug"
)

// Guard runs fn and keeps any failure inside the plugin: panics are recovered,
// panics and errors are logged with the hook name, and the caller always gets a
// result instead of a crash, so that Bifrost keeps serving the request. It
// returns true when fn returned nil without panicking.
func Guard(logger *slog.Logger, name string, fn func() error) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("freshness-guard hook failed", "hook", name, "panic", r, "stack", string(debug.Stack()))
			ok = false
		}
	}()
	if err := fn(); err != nil {
		logger.Error("freshness-guard hook failed", "hook", name, "error", err)
		return false
	}
	return true
}
