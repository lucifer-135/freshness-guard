package hooks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lucifer-135/freshness-guard/internal/config"
	"github.com/lucifer-135/freshness-guard/internal/hooks"
)

func TestSelectMode(t *testing.T) {
	withKeys := config.Default()
	withKeys.VirtualKeys = map[string]config.VirtualKey{
		"vk-demo": {Mode: "sync", SampleRate: 1.0},
		"vk-s":    {Mode: "async", SampleRate: 0.25},
	}
	offByDefault := config.Default()
	offByDefault.DefaultMode = "off"

	cases := []struct {
		name, header, vk string
		cfg              config.Config
		sample           float64
		want             hooks.Mode
	}{
		{"header wins, any case", "SYNC", "", config.Default(), 0, hooks.ModeSync},
		{"header off beats virtual key", "off", "vk-demo", withKeys, 0, hooks.ModeOff},
		{"invalid header falls through", "bogus", "", config.Default(), 0, hooks.ModeAsync},
		{"virtual key mode", "", "vk-demo", withKeys, 0.99, hooks.ModeSync},
		{"virtual key not sampled", "", "vk-s", withKeys, 0.5, hooks.ModeOff},
		{"virtual key sampled", "", "vk-s", withKeys, 0.1, hooks.ModeAsync},
		{"unknown virtual key uses default", "", "vk-other", withKeys, 0, hooks.ModeAsync},
		{"default mode", "", "", offByDefault, 0, hooks.ModeOff},
		{"header trimmed", "  async ", "", offByDefault, 0, hooks.ModeAsync},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hooks.SelectMode(tc.header, tc.vk, tc.cfg, tc.sample))
		})
	}
}
