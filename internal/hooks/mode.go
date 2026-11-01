package hooks

import (
	"strings"

	"github.com/lucifer-135/freshness-guard/internal/config"
)

// Mode says whether and how a request is checked.
type Mode string

// Modes.
const (
	ModeAsync Mode = "async"
	ModeSync  Mode = "sync"
	ModeOff   Mode = "off"
)

// Request headers read by the plugin.
const (
	HeaderFreshness  = "x-bf-freshness"
	HeaderVirtualKey = "x-bf-vk"
)

// SelectMode picks the mode for a request. A valid x-bf-freshness header wins;
// then the virtual key's settings, where a sample at or above the sample rate
// turns checking off; then the configured default.
func SelectMode(header, virtualKey string, cfg config.Config, sample float64) Mode {
	switch m := Mode(strings.ToLower(strings.TrimSpace(header))); m {
	case ModeAsync, ModeSync, ModeOff:
		return m
	}
	if vk, ok := cfg.VirtualKeys[virtualKey]; ok && virtualKey != "" {
		if sample >= vk.SampleRate {
			return ModeOff
		}
		return Mode(vk.Mode)
	}
	return Mode(cfg.DefaultMode)
}

// Policy says what synchronous mode does with an answer that has outdated
// claims.
type Policy string

// Policies.
const (
	PolicyAnnotate Policy = "annotate"
	PolicyHeal     Policy = "heal"
	PolicyBlock    Policy = "block"
)

// HeaderPolicy is the request header that selects the policy.
const HeaderPolicy = "x-bf-freshness-policy"

// SelectPolicy picks the policy for a request: a valid x-bf-freshness-policy
// header wins, then the virtual key's policy, then the configured default.
func SelectPolicy(header, virtualKey string, cfg config.Config) Policy {
	switch p := Policy(strings.ToLower(strings.TrimSpace(header))); p {
	case PolicyAnnotate, PolicyHeal, PolicyBlock:
		return p
	}
	if vk, ok := cfg.VirtualKeys[virtualKey]; ok && virtualKey != "" && vk.Policy != "" {
		return Policy(vk.Policy)
	}
	return Policy(cfg.DefaultPolicy)
}
