// Package app assembles the plugin's parts from the configuration and owns
// their lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	"github.com/lucifer-135/freshness-guard/internal/api"
	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/engine"
	"github.com/lucifer-135/freshness-guard/internal/extract"
	"github.com/lucifer-135/freshness-guard/internal/facts"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
	"github.com/lucifer-135/freshness-guard/internal/llm"
	"github.com/lucifer-135/freshness-guard/internal/store"
	"github.com/lucifer-135/freshness-guard/internal/tavily"
	"github.com/lucifer-135/freshness-guard/internal/verify"
)

const (
	engineStopTimeout = 10 * time.Second
	apiStopTimeout    = 5 * time.Second
)

// App is one running instance of the plugin.
type App struct {
	Config config.Config
	Hooks  *hooks.Handler

	store    *store.SQLite
	engine   *engine.Engine
	server   *api.Server
	serverUp bool
	logger   *slog.Logger
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// New builds the plugin from cfg. It fails only when the database cannot be
// opened.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	st, err := store.OpenSQLite(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open the freshness database: %w", err)
	}
	if cfg.TavilyAPIKey == "" {
		logger.Warn("tavily_api_key is empty, so checked claims will be marked as could not verify.")
	}
	if cfg.APIToken == "" {
		logger.Warn("api.token is empty, so the protected plugin API routes are disabled.")
	}

	llmClient := llm.New(cfg.BifrostBaseURL, cfg.BifrostVirtualKey)
	budget := engine.NewBudget(st, cfg.DailyCreditBudget, time.Now, logger)
	pipeline := &engine.Pipeline{
		Extract: &extract.Extractor{LLM: llmClient, Model: cfg.ExtractModel, Fallbacks: cfg.ExtractFallbacks, MaxClaims: cfg.MaxClaims},
		Verify: &verify.Verifier{Search: tavily.New(cfg.TavilyBaseURL, cfg.TavilyAPIKey), LLM: llmClient,
			Model: cfg.JudgeModel, Fallbacks: cfg.JudgeFallbacks, Budget: budget, Now: time.Now},
		Facts:         &facts.Cache{Store: st, Now: time.Now},
		Store:         st,
		Budget:        budget,
		Cutoffs:       cfg.ModelCutoffs,
		RiskThreshold: cfg.RiskThreshold,
		Now:           time.Now,
		Logger:        logger,
	}
	eng := engine.New(pipeline, cfg.Workers, cfg.QueueSize, logger)

	handler := api.NewHandler(api.Deps{
		Token: cfg.APIToken, Reports: st, Jobs: eng, NewID: newID, Now: time.Now,
		CheckTimeout: api.DefaultCheckTimeout, Logger: logger,
	})
	return &App{
		Config: cfg,
		Hooks: &hooks.Handler{Config: cfg, Jobs: eng, NewID: newID, Now: time.Now, Sample: rand.Float64,
			Logger: logger},
		store:  st,
		engine: eng,
		server: api.NewServer(fmt.Sprintf("%s:%d", cfg.APIHost, cfg.APIPort), handler, logger),
		logger: logger,
	}, nil
}

// Start starts the workers and then the API server. It returns the API
// server's listen error; the pipeline keeps working without the API.
func (a *App) Start() error {
	a.engine.Start()
	if err := a.server.Start(); err != nil {
		return fmt.Errorf("start the plugin API on port %d: %w", a.Config.APIPort, err)
	}
	a.serverUp = true
	return nil
}

// Stop shuts down the API server, drains the workers and closes the database.
func (a *App) Stop(ctx context.Context) error {
	var errs []error
	if a.serverUp {
		apiCtx, cancel := context.WithTimeout(ctx, apiStopTimeout)
		errs = append(errs, a.server.Shutdown(apiCtx))
		cancel()
		a.serverUp = false
	}
	engCtx, cancel := context.WithTimeout(ctx, engineStopTimeout)
	errs = append(errs, a.engine.Stop(engCtx))
	cancel()
	errs = append(errs, a.store.Close())
	return errors.Join(errs...)
}
