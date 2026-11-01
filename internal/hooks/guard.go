// Package hooks holds the logic behind the plugin's exported Bifrost hooks.
package hooks

import "log/slog"

// Guard runs fn and keeps any failure inside the plugin: panics are recovered,
// panics and errors are logged with the hook name, and the result is always nil
// so that Bifrost keeps serving the request.
func Guard(logger *slog.Logger, name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("freshness-guard hook failed", "hook", name, "panic", r)
			err = nil
		}
	}()
	if fnErr := fn(); fnErr != nil {
		logger.Error("freshness-guard hook failed", "hook", name, "error", fnErr)
	}
	return nil
}
