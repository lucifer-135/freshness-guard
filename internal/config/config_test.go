package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/config"
)

func TestParseNilUsesDefaults(t *testing.T) {
	cfg, warnings := config.Parse(nil)
	assert.Equal(t, 8090, cfg.APIPort)
	assert.Equal(t, "", cfg.APIToken)
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
