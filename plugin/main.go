// Command plugin is the Freshness Guard plugin for the Bifrost gateway. It is
// built with -buildmode=plugin; Bifrost looks up the exported functions by name.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

const pluginName = "freshness-guard"

var (
	mu     sync.Mutex
	server *api.Server
	logger = slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("plugin", pluginName)
)

// Init parses the plugin config and starts the API server. It always returns
// nil so that a plugin problem never stops Bifrost from starting.
func Init(raw any) error {
	cfg, warnings := config.Parse(raw)
	for _, w := range warnings {
		logger.Warn(w)
	}

	srv := api.NewServer(fmt.Sprintf("0.0.0.0:%d", cfg.APIPort))
	if err := srv.Start(); err != nil {
		logger.Error("freshness-guard API server did not start", "port", cfg.APIPort, "error", err)
		return nil
	}
	mu.Lock()
	server = srv
	mu.Unlock()
	logger.Info("freshness-guard API server started", "port", cfg.APIPort)
	return nil
}

// GetName returns the plugin name.
func GetName() string {
	return pluginName
}

// Cleanup stops the API server.
func Cleanup() error {
	mu.Lock()
	srv := server
	server = nil
	mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// HTTPTransportResponseHeadersHook adds the X-Freshness-Status header.
func HTTPTransportResponseHeadersHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, resp *schemas.HTTPResponseMetadata) error {
	return hooks.Guard(logger, "HTTPTransportResponseHeadersHook", func() error {
		hooks.SetStatusHeader(resp, hooks.StatusSkipped)
		return nil
	})
}

func main() {}
