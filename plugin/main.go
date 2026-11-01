// Command plugin is the Freshness Guard plugin for the Bifrost gateway. It is
// built with -buildmode=plugin; Bifrost looks up the exported functions by name.
package main

import (
	"context"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"

	"github.com/lucifer-135/freshness-guard/internal/app"
	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

const (
	pluginName  = "freshness-guard"
	stopTimeout = 15 * time.Second
)

// Bifrost reloads a plugin at runtime by calling Init again on the same loaded
// .so before it retires the old instance, so the package state is shared and
// Init/Cleanup calls are reference-counted.
var (
	mu         sync.RWMutex
	current    *app.App
	currentCfg config.Config
	refs       int
	logger     = slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("plugin", pluginName)
)

func getApp() *app.App {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Init parses the plugin config and starts the plugin, or keeps the running
// one when the config is unchanged. It always returns nil so that a plugin
// problem never stops Bifrost from starting.
func Init(raw any) error {
	cfg, warnings := config.Parse(raw)
	for _, w := range warnings {
		logger.Warn(w)
	}

	mu.Lock()
	defer mu.Unlock()
	refs++
	if current != nil && reflect.DeepEqual(currentCfg, cfg) {
		return nil
	}
	if current != nil {
		stop(current)
		current = nil
	}

	a, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("Freshness Guard could not start, so answers pass through unchecked.", "error", err)
		return nil
	}
	if err := a.Start(); err != nil {
		logger.Error("freshness-guard API server did not start", "port", cfg.APIPort, "error", err)
	} else {
		logger.Info("freshness-guard API server started", "port", cfg.APIPort)
	}
	current, currentCfg = a, cfg
	return nil
}

func stop(a *app.App) {
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := a.Stop(ctx); err != nil {
		logger.Error("Freshness Guard did not stop cleanly.", "error", err)
	}
}

// GetName returns the plugin name.
func GetName() string {
	return pluginName
}

// Cleanup stops the plugin when the last Init has been paired with a Cleanup.
func Cleanup() error {
	mu.Lock()
	defer mu.Unlock()
	if refs > 0 {
		refs--
	}
	if refs > 0 || current == nil {
		return nil
	}
	stop(current)
	current = nil
	return nil
}

// guard runs fn inside hooks.Guard and marks the request as failed when fn
// panics.
func guard(ctx *schemas.BifrostContext, name string, fn func()) {
	if !hooks.Guard(logger, name, func() error { fn(); return nil }) {
		hooks.MarkFailed(ctx)
	}
}

// HTTPTransportPreHook selects the mode for the request.
func HTTPTransportPreHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	if a := getApp(); a != nil {
		guard(ctx, "HTTPTransportPreHook", func() { a.Hooks.TransportPre(ctx, req) })
	}
	return nil, nil
}

// PreLLMHook records the prompt and model. The request passes through unchanged.
func PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if a := getApp(); a != nil {
		guard(ctx, "PreLLMHook", func() { a.Hooks.PreLLM(ctx, req) })
	}
	return req, nil, nil
}

// PostLLMHook submits the answer for verification. The response passes
// through unchanged.
func PostLLMHook(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	if a := getApp(); a != nil {
		guard(ctx, "PostLLMHook", func() { a.Hooks.PostLLM(ctx, resp) })
	}
	return resp, bifrostErr, nil
}

// HTTPTransportResponseHeadersHook adds the X-Freshness-* headers.
func HTTPTransportResponseHeadersHook(ctx *schemas.BifrostContext, _ *schemas.HTTPRequest, resp *schemas.HTTPResponseMetadata) error {
	a := getApp()
	if a == nil {
		hooks.SetStatusHeader(resp, hooks.StatusError)
		return nil
	}
	ok := hooks.Guard(logger, "HTTPTransportResponseHeadersHook", func() error {
		a.Hooks.ResponseHeaders(ctx, resp)
		return nil
	})
	if !ok {
		hooks.SetStatusHeader(resp, hooks.StatusError)
	}
	return nil
}

func main() {}
