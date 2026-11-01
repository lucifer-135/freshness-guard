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

// Bifrost reloads a plugin at runtime by calling Init again on the same loaded
// .so before it retires the old instance, so the package state is shared and
// Init/Cleanup calls are reference-counted.
var (
	mu         sync.Mutex
	server     *api.Server
	serverPort int
	refs       int
	logger     = slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("plugin", pluginName)
)

// Init parses the plugin config and starts the API server, or keeps the running
// one when the port is unchanged. It always returns nil so that a plugin
// problem never stops Bifrost from starting.
func Init(raw any) error {
	cfg, warnings := config.Parse(raw)
	for _, w := range warnings {
		logger.Warn(w)
	}

	mu.Lock()
	defer mu.Unlock()
	refs++
	if server != nil && serverPort == cfg.APIPort {
		return nil
	}
	if server != nil {
		shutdown(server)
		server = nil
	}

	srv := api.NewServer(fmt.Sprintf("0.0.0.0:%d", cfg.APIPort))
	if err := srv.Start(); err != nil {
		logger.Error("freshness-guard API server did not start", "port", cfg.APIPort, "error", err)
		return nil
	}
	server, serverPort = srv, cfg.APIPort
	logger.Info("freshness-guard API server started", "port", cfg.APIPort)
	return nil
}

func shutdown(srv *api.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("freshness-guard API server did not stop cleanly", "error", err)
	}
}

// GetName returns the plugin name.
func GetName() string {
	return pluginName
}

// Cleanup stops the API server when the last Init has been paired with a Cleanup.
func Cleanup() error {
	mu.Lock()
	defer mu.Unlock()
	if refs > 0 {
		refs--
	}
	if refs > 0 || server == nil {
		return nil
	}
	shutdown(server)
	server = nil
	return nil
}

// HTTPTransportResponseHeadersHook adds the X-Freshness-Status header.
func HTTPTransportResponseHeadersHook(_ *schemas.BifrostContext, _ *schemas.HTTPRequest, resp *schemas.HTTPResponseMetadata) error {
	return hooks.Guard(logger, "HTTPTransportResponseHeadersHook", func() error {
		hooks.SetStatusHeader(resp, hooks.StatusSkipped)
		return nil
	})
}

func main() {}
