// Package config parses the plugin configuration block from Bifrost's config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIPort     = 8090
	defaultBifrostPort = 8080
)

// VirtualKey holds the per-virtual-key settings.
type VirtualKey struct {
	Mode       string
	SampleRate float64
	// Policy is empty when the key uses DefaultPolicy.
	Policy string
}

// Config is the validated plugin configuration.
type Config struct {
	APIHost, APIToken string
	APIPort           int

	BifrostBaseURL    string
	BifrostVirtualKey string

	TavilyAPIKey, TavilyBaseURL string

	DefaultMode   string
	DefaultPolicy string

	// Synchronous mode waits at most SyncTimeout for verification. The block
	// policy blocks claims judged STALE or WRONG with at least BlockConfidence;
	// without a complete result it blocks only when FailClosed is set.
	SyncTimeout     time.Duration
	BlockConfidence float64
	FailClosed      bool

	ExtractModel, JudgeModel, HealModel             string
	ExtractFallbacks, JudgeFallbacks, HealFallbacks []string

	ModelCutoffs map[string]time.Time

	RiskThreshold                                    float64
	MaxClaims, Workers, QueueSize, DailyCreditBudget int

	DBPath string

	VirtualKeys map[string]VirtualKey
}

// Default returns the configuration used when no valid value is given.
func Default() Config {
	return Config{
		APIHost:           "0.0.0.0",
		APIPort:           defaultAPIPort,
		BifrostBaseURL:    DefaultBifrostBaseURL(os.Args),
		TavilyBaseURL:     "https://api.tavily.com",
		DefaultMode:       "async",
		DefaultPolicy:     "heal",
		SyncTimeout:       8 * time.Second,
		BlockConfidence:   0.7,
		ExtractModel:      "groq/openai/gpt-oss-20b",
		JudgeModel:        "groq/openai/gpt-oss-120b",
		HealModel:         "groq/openai/gpt-oss-120b",
		ExtractFallbacks:  []string{"gemini/gemini-3.5-flash-lite"},
		JudgeFallbacks:    []string{"gemini/gemini-3.5-flash"},
		HealFallbacks:     []string{"gemini/gemini-3.5-flash"},
		RiskThreshold:     0.3,
		MaxClaims:         12,
		Workers:           4,
		QueueSize:         256,
		DailyCreditBudget: 30,
		DBPath:            "/app/data/freshness.db",
	}
}

type object = map[string]json.RawMessage

// parser collects warnings while each field is decoded on its own, so that one
// bad field does not reset the others.
type parser struct {
	warnings []string
}

func (p *parser) warn(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

// decode reads obj[key] into dst. It returns false when the key is absent or
// has the wrong type; dst is unchanged in both cases.
func decode[T any](p *parser, obj object, path, key string, dst *T) bool {
	raw, ok := obj[key]
	if !ok {
		return false
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		p.warn("plugin config field %q has the wrong type, so the default is used: %v", path+key, err)
		return false
	}
	*dst = v
	return true
}

// str reads a string field, resolves an env. reference, and keeps the default
// when required is true and the result is empty.
func (p *parser) str(obj object, path, key string, dst *string, required bool) {
	var v string
	if !decode(p, obj, path, key, &v) {
		return
	}
	if name, ok := strings.CutPrefix(v, "env."); ok {
		v = os.Getenv(name)
		if v == "" {
			p.warn("plugin config field %q refers to environment variable %s, which is empty", path+key, name)
		}
	} else if v == "" && required {
		p.warn("plugin config field %q must not be empty, so the default is used", path+key)
	}
	if v == "" && required {
		return
	}
	*dst = v
}

func (p *parser) intRange(obj object, key string, dst *int, lo, hi int) {
	var v int
	if !decode(p, obj, "", key, &v) {
		return
	}
	if v < lo || v > hi {
		p.warn("%s %d is outside %d-%d, so the default %d is used", key, v, lo, hi, *dst)
		return
	}
	*dst = v
}

func validMode(m string) bool {
	return m == "async" || m == "sync" || m == "off"
}

func validPolicy(p string) bool {
	return p == "annotate" || p == "heal" || p == "block"
}

// DefaultBifrostBaseURL returns the address of the Bifrost instance the
// plugin runs in, on the port read from Bifrost's arguments.
func DefaultBifrostBaseURL(args []string) string {
	return fmt.Sprintf("http://127.0.0.1:%d", BifrostPort(args))
}

// Parse converts the raw plugin config into a Config. It never fails: invalid
// input falls back to defaults and is reported in the returned warnings.
func Parse(raw any) (Config, []string) {
	cfg := Default()
	if raw == nil {
		return cfg, nil
	}

	var obj object
	data, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(data, &obj)
	}
	if err != nil {
		return Default(), []string{fmt.Sprintf("plugin config is not a valid object, using defaults: %v", err)}
	}

	p := &parser{}
	p.parseAPI(obj, &cfg)

	p.str(obj, "", "bifrost_base_url", &cfg.BifrostBaseURL, true)
	p.str(obj, "", "bifrost_virtual_key", &cfg.BifrostVirtualKey, false)
	p.str(obj, "", "tavily_api_key", &cfg.TavilyAPIKey, false)
	p.str(obj, "", "tavily_base_url", &cfg.TavilyBaseURL, true)
	p.str(obj, "", "db_path", &cfg.DBPath, true)

	var mode string
	if decode(p, obj, "", "default_mode", &mode) {
		if validMode(mode) {
			cfg.DefaultMode = mode
		} else {
			p.warn("default_mode %q is not async, sync or off, so %q is used", mode, cfg.DefaultMode)
		}
	}

	var policy string
	if decode(p, obj, "", "default_policy", &policy) {
		if validPolicy(policy) {
			cfg.DefaultPolicy = policy
		} else {
			p.warn("default_policy %q is not annotate, heal or block, so %q is used", policy, cfg.DefaultPolicy)
		}
	}
	syncMS := int(cfg.SyncTimeout / time.Millisecond)
	p.intRange(obj, "sync_timeout_ms", &syncMS, 1000, 60000)
	cfg.SyncTimeout = time.Duration(syncMS) * time.Millisecond
	var blockConfidence float64
	if decode(p, obj, "", "block_confidence", &blockConfidence) {
		if blockConfidence > 0 && blockConfidence <= 1 {
			cfg.BlockConfidence = blockConfidence
		} else {
			p.warn("block_confidence %v must be greater than 0 and at most 1, so the default %v is used", blockConfidence, cfg.BlockConfidence)
		}
	}
	decode(p, obj, "", "fail_closed", &cfg.FailClosed)

	var models object
	if decode(p, obj, "", "models", &models) {
		p.str(models, "models.", "extract", &cfg.ExtractModel, true)
		p.str(models, "models.", "judge", &cfg.JudgeModel, true)
		p.str(models, "models.", "heal", &cfg.HealModel, true)
	}
	var fallbacks object
	if decode(p, obj, "", "fallbacks", &fallbacks) {
		decode(p, fallbacks, "fallbacks.", "extract", &cfg.ExtractFallbacks)
		decode(p, fallbacks, "fallbacks.", "judge", &cfg.JudgeFallbacks)
		decode(p, fallbacks, "fallbacks.", "heal", &cfg.HealFallbacks)
	}

	var threshold float64
	if decode(p, obj, "", "risk_threshold", &threshold) {
		if threshold > 0 && threshold <= 1 {
			cfg.RiskThreshold = threshold
		} else {
			p.warn("risk_threshold %v must be greater than 0 and at most 1, so the default %v is used", threshold, cfg.RiskThreshold)
		}
	}
	p.intRange(obj, "max_claims", &cfg.MaxClaims, 1, 50)
	p.intRange(obj, "workers", &cfg.Workers, 1, 64)
	p.intRange(obj, "queue_size", &cfg.QueueSize, 1, 10000)
	p.intRange(obj, "daily_credit_budget", &cfg.DailyCreditBudget, 0, 100000)

	p.parseCutoffs(obj, &cfg)
	p.parseVirtualKeys(obj, &cfg)
	return cfg, p.warnings
}

func (p *parser) parseAPI(obj object, cfg *Config) {
	var api object
	if !decode(p, obj, "", "api", &api) {
		return
	}
	p.str(api, "api.", "host", &cfg.APIHost, true)
	p.str(api, "api.", "token", &cfg.APIToken, false)

	var port int
	if !decode(p, api, "api.", "port", &port) {
		return
	}
	switch bifrostPort := BifrostPort(os.Args); {
	case port < 1 || port > 65535:
		p.warn("api.port %d is outside 1-65535, using %d", port, defaultAPIPort)
	case port == bifrostPort:
		p.warn("api.port %d is Bifrost's own port, so %d is used instead", port, defaultAPIPort)
	default:
		cfg.APIPort = port
	}
}

func (p *parser) parseCutoffs(obj object, cfg *Config) {
	var cutoffs map[string]string
	if !decode(p, obj, "", "model_cutoffs", &cutoffs) {
		return
	}
	cfg.ModelCutoffs = make(map[string]time.Time, len(cutoffs))
	for model, value := range cutoffs {
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			t, err = time.Parse("2006-01", value)
		}
		if err != nil {
			p.warn("model_cutoffs entry %q has the date %q, which is not YYYY-MM or YYYY-MM-DD, so it is ignored", model, value)
			continue
		}
		cfg.ModelCutoffs[model] = t.UTC()
	}
}

func (p *parser) parseVirtualKeys(obj object, cfg *Config) {
	var keys object
	if !decode(p, obj, "", "virtual_keys", &keys) {
		return
	}
	cfg.VirtualKeys = make(map[string]VirtualKey, len(keys))
	for key, raw := range keys {
		var entry struct {
			Mode       string   `json:"mode"`
			SampleRate *float64 `json:"sample_rate"`
			Policy     string   `json:"policy"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			p.warn("virtual_keys entry %q has the wrong type, so it is ignored: %v", key, err)
			continue
		}
		if !validMode(entry.Mode) {
			p.warn("virtual_keys entry %q has mode %q, which is not async, sync or off, so it is ignored", key, entry.Mode)
			continue
		}
		rate := 1.0
		if entry.SampleRate != nil {
			rate = *entry.SampleRate
		}
		if rate < 0 || rate > 1 {
			p.warn("virtual_keys entry %q has sample_rate %v, which is outside 0-1, so it is ignored", key, rate)
			continue
		}
		if entry.Policy != "" && !validPolicy(entry.Policy) {
			p.warn("virtual_keys entry %q has policy %q, which is not annotate, heal or block, so it is ignored", key, entry.Policy)
			continue
		}
		cfg.VirtualKeys[key] = VirtualKey{Mode: entry.Mode, SampleRate: rate, Policy: entry.Policy}
	}
}

// BifrostPort returns the port Bifrost listens on, read from its command-line
// arguments (-port N, --port N, -port=N or --port=N). The default is 8080.
func BifrostPort(args []string) int {
	for i, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "-port" && name != "--port" {
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				break
			}
			value = args[i+1]
		}
		if port, err := strconv.Atoi(value); err == nil {
			return port
		}
	}
	return defaultBifrostPort
}

// ResolveEnv returns the value of environment variable X for "env.X", and the
// value unchanged otherwise.
func ResolveEnv(value string) string {
	if name, ok := strings.CutPrefix(value, "env."); ok {
		return os.Getenv(name)
	}
	return value
}
