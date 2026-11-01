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
