package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/config"
)

func TestParseNilUsesDefaults(t *testing.T) {
	cfg, warnings := config.Parse(nil)
	assert.Equal(t, config.Default(), cfg)
	assert.Empty(t, warnings)
}

func TestParseReadsPortAndResolvesEnvToken(t *testing.T) {
	t.Setenv("FG_API_TOKEN", "secret-value")
	raw := map[string]any{"api": map[string]any{"port": float64(9100), "token": "env.FG_API_TOKEN"}}
	cfg, warnings := config.Parse(raw)
	assert.Equal(t, 9100, cfg.APIPort)
	assert.Equal(t, "secret-value", cfg.APIToken)
	assert.Empty(t, warnings)
}

func TestParseRejectsOutOfRangePort(t *testing.T) {
	for _, port := range []float64{0, -1, 70000} {
		cfg, warnings := config.Parse(map[string]any{"api": map[string]any{"port": port}})
		assert.Equal(t, 8090, cfg.APIPort)
		assert.Len(t, warnings, 1)
	}
}

func TestParseWrongTypeFallsBackToDefaults(t *testing.T) {
	cfg, warnings := config.Parse("not-an-object")
	assert.Equal(t, config.Default(), cfg)
	assert.Len(t, warnings, 1)
}

func TestResolveEnvUnsetVariableIsEmpty(t *testing.T) {
	assert.Equal(t, "", config.ResolveEnv("env.FG_DEFINITELY_UNSET_123"))
	assert.Equal(t, "plain", config.ResolveEnv("plain"))
}

func TestDefaultsMatchSpec(t *testing.T) {
	d := config.Default()
	assert.Equal(t, "0.0.0.0", d.APIHost)
	assert.Equal(t, 8090, d.APIPort)
	assert.Equal(t, "http://127.0.0.1:8080", d.BifrostBaseURL)
	assert.Equal(t, "https://api.tavily.com", d.TavilyBaseURL)
	assert.Equal(t, "async", d.DefaultMode)
	assert.Equal(t, "groq/openai/gpt-oss-20b", d.ExtractModel)
	assert.Equal(t, "groq/openai/gpt-oss-120b", d.JudgeModel)
	assert.Equal(t, []string{"gemini/gemini-3.5-flash-lite"}, d.ExtractFallbacks)
	assert.Equal(t, []string{"gemini/gemini-3.5-flash"}, d.JudgeFallbacks)
	assert.Equal(t, 0.3, d.RiskThreshold)
	assert.Equal(t, 12, d.MaxClaims)
	assert.Equal(t, 4, d.Workers)
	assert.Equal(t, 256, d.QueueSize)
	assert.Equal(t, 30, d.DailyCreditBudget)
	assert.Equal(t, "/app/data/freshness.db", d.DBPath)
}

func TestParseWrongTypeForOneFieldKeepsOthers(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"workers": "four", "max_claims": float64(5)})
	assert.Equal(t, 4, cfg.Workers)
	assert.Equal(t, 5, cfg.MaxClaims)
	require.Len(t, w, 1)
	assert.Contains(t, w[0], "workers")
}

func TestParseWarnsOnEmptyEnvReference(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"tavily_api_key": "env.FG_UNSET_XYZ"})
	assert.Equal(t, "", cfg.TavilyAPIKey)
	require.Len(t, w, 1)
	assert.Contains(t, w[0], "FG_UNSET_XYZ")
}

func TestParseRejectsBifrostPort(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"api": map[string]any{"port": float64(8080)}})
	assert.Equal(t, 8090, cfg.APIPort)
	require.Len(t, w, 1)
	assert.Contains(t, w[0], "8080")
}

func TestBifrostPortFromArgs(t *testing.T) {
	assert.Equal(t, 8080, config.BifrostPort(nil))
	assert.Equal(t, 9000, config.BifrostPort([]string{"bifrost-http", "-port", "9000"}))
	assert.Equal(t, 9001, config.BifrostPort([]string{"x", "--port=9001"}))
	assert.Equal(t, 8080, config.BifrostPort([]string{"x", "-port", "abc"}))
}

func TestParseModelCutoffs(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"model_cutoffs": map[string]any{
		"groq/openai/gpt-oss-20b": "2024-06", "bad/model": "June"}})
	assert.Equal(t, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), cfg.ModelCutoffs["groq/openai/gpt-oss-20b"])
	assert.NotContains(t, cfg.ModelCutoffs, "bad/model")
	require.Len(t, w, 1)
	assert.Contains(t, w[0], "bad/model")
}

func TestParseVirtualKeys(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"virtual_keys": map[string]any{
		"vk-demo": map[string]any{"mode": "sync", "sample_rate": 0.5},
		"vk-bad":  map[string]any{"mode": "fast"},
	}})
	assert.Equal(t, config.VirtualKey{Mode: "sync", SampleRate: 0.5}, cfg.VirtualKeys["vk-demo"])
	assert.NotContains(t, cfg.VirtualKeys, "vk-bad")
	assert.Len(t, w, 1)
}

func TestParseVirtualKeyDefaultSampleRate(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"virtual_keys": map[string]any{"vk": map[string]any{"mode": "async"}}})
	assert.Equal(t, config.VirtualKey{Mode: "async", SampleRate: 1.0}, cfg.VirtualKeys["vk"])
	assert.Empty(t, w)
}

func TestParseRejectsOutOfRangeValues(t *testing.T) {
	d := config.Default()
	cases := []struct {
		name  string
		raw   map[string]any
		check func(config.Config) bool
	}{
		{"risk_threshold", map[string]any{"risk_threshold": 0.0}, func(c config.Config) bool { return c.RiskThreshold == d.RiskThreshold }},
		{"max_claims", map[string]any{"max_claims": 0.0}, func(c config.Config) bool { return c.MaxClaims == d.MaxClaims }},
		{"queue_size", map[string]any{"queue_size": -1.0}, func(c config.Config) bool { return c.QueueSize == d.QueueSize }},
		{"workers", map[string]any{"workers": 65.0}, func(c config.Config) bool { return c.Workers == d.Workers }},
		{"default_mode", map[string]any{"default_mode": "fast"}, func(c config.Config) bool { return c.DefaultMode == "async" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, w := config.Parse(tc.raw)
			assert.True(t, tc.check(cfg))
			require.Len(t, w, 1)
			assert.Contains(t, w[0], tc.name)
		})
	}
}

func TestParseReadsAllFields(t *testing.T) {
	cfg, w := config.Parse(map[string]any{
		"api":                 map[string]any{"host": "127.0.0.1", "port": 9200.0, "token": "t"},
		"bifrost_base_url":    "http://b:8080",
		"bifrost_virtual_key": "vk",
		"tavily_api_key":      "k",
		"tavily_base_url":     "http://t",
		"default_mode":        "off",
		"models":              map[string]any{"extract": "a/x", "judge": "a/y"},
		"fallbacks":           map[string]any{"extract": []any{}, "judge": []any{"b/z"}},
		"risk_threshold":      0.5,
		"max_claims":          3.0,
		"workers":             2.0,
		"queue_size":          10.0,
		"daily_credit_budget": 0.0,
		"db_path":             "/tmp/f.db",
	})
	assert.Empty(t, w)
	assert.Equal(t, "127.0.0.1", cfg.APIHost)
	assert.Equal(t, 9200, cfg.APIPort)
	assert.Equal(t, "t", cfg.APIToken)
	assert.Equal(t, "http://b:8080", cfg.BifrostBaseURL)
	assert.Equal(t, "vk", cfg.BifrostVirtualKey)
	assert.Equal(t, "k", cfg.TavilyAPIKey)
	assert.Equal(t, "http://t", cfg.TavilyBaseURL)
	assert.Equal(t, "off", cfg.DefaultMode)
	assert.Equal(t, "a/x", cfg.ExtractModel)
	assert.Equal(t, "a/y", cfg.JudgeModel)
	assert.Empty(t, cfg.ExtractFallbacks)
	assert.Equal(t, []string{"b/z"}, cfg.JudgeFallbacks)
	assert.Equal(t, 0.5, cfg.RiskThreshold)
	assert.Equal(t, 3, cfg.MaxClaims)
	assert.Equal(t, 2, cfg.Workers)
	assert.Equal(t, 10, cfg.QueueSize)
	assert.Equal(t, 0, cfg.DailyCreditBudget)
	assert.Equal(t, "/tmp/f.db", cfg.DBPath)
}

func TestDefaultsForStrictMode(t *testing.T) {
	d := config.Default()
	assert.Equal(t, "heal", d.DefaultPolicy)
	assert.Equal(t, 8*time.Second, d.SyncTimeout)
	assert.Equal(t, 0.7, d.BlockConfidence)
	assert.False(t, d.FailClosed)
	assert.Equal(t, "groq/openai/gpt-oss-120b", d.HealModel)
	assert.Equal(t, []string{"gemini/gemini-3.5-flash"}, d.HealFallbacks)
	assert.Equal(t, "http://127.0.0.1:8080", d.BifrostBaseURL)
}

func TestDefaultBifrostBaseURLFollowsPort(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:9000", config.DefaultBifrostBaseURL([]string{"x", "-port", "9000"}))
	assert.Equal(t, "http://127.0.0.1:8080", config.DefaultBifrostBaseURL(nil))
}

func TestParseStrictModeFields(t *testing.T) {
	cfg, w := config.Parse(map[string]any{
		"default_policy": "block", "sync_timeout_ms": 3000.0, "block_confidence": 0.9, "fail_closed": true,
		"models": map[string]any{"heal": "a/h"}, "fallbacks": map[string]any{"heal": []any{}},
	})
	assert.Empty(t, w)
	assert.Equal(t, "block", cfg.DefaultPolicy)
	assert.Equal(t, 3*time.Second, cfg.SyncTimeout)
	assert.Equal(t, 0.9, cfg.BlockConfidence)
	assert.True(t, cfg.FailClosed)
	assert.Equal(t, "a/h", cfg.HealModel)
	assert.Empty(t, cfg.HealFallbacks)
}

func TestParseRejectsBadStrictModeValues(t *testing.T) {
	d := config.Default()
	cases := []struct {
		key   string
		value any
		check func(config.Config) bool
	}{
		{"default_policy", "fix", func(c config.Config) bool { return c.DefaultPolicy == d.DefaultPolicy }},
		{"sync_timeout_ms", 500.0, func(c config.Config) bool { return c.SyncTimeout == d.SyncTimeout }},
		{"block_confidence", 0.0, func(c config.Config) bool { return c.BlockConfidence == d.BlockConfidence }},
		{"fail_closed", "yes", func(c config.Config) bool { return c.FailClosed == d.FailClosed }},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			cfg, w := config.Parse(map[string]any{tc.key: tc.value})
			assert.True(t, tc.check(cfg))
			require.Len(t, w, 1)
			assert.Contains(t, w[0], tc.key)
		})
	}
}

func TestParseVirtualKeyPolicy(t *testing.T) {
	cfg, w := config.Parse(map[string]any{"virtual_keys": map[string]any{
		"vk-a": map[string]any{"mode": "sync", "policy": "block"},
		"vk-b": map[string]any{"mode": "sync", "policy": "fix"},
	}})
	assert.Equal(t, config.VirtualKey{Mode: "sync", SampleRate: 1, Policy: "block"}, cfg.VirtualKeys["vk-a"])
	assert.NotContains(t, cfg.VirtualKeys, "vk-b")
	assert.Len(t, w, 1)
}
