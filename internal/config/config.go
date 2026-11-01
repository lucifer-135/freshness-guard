// Package config parses the plugin configuration block from Bifrost's config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const defaultAPIPort = 8090

// Config is the validated plugin configuration.
type Config struct {
	APIPort  int
	APIToken string
}

type rawConfig struct {
	API struct {
		Port  *int   `json:"port"`
		Token string `json:"token"`
	} `json:"api"`
}

// Default returns the configuration used when no valid value is given.
func Default() Config {
	return Config{APIPort: defaultAPIPort}
}

// Parse converts the raw plugin config into a Config. It never fails: invalid
// input falls back to defaults and is reported in the returned warnings.
func Parse(raw any) (Config, []string) {
	cfg := Default()
	if raw == nil {
		return cfg, nil
	}

	var parsed rawConfig
	data, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(data, &parsed)
	}
	if err != nil {
		return Default(), []string{fmt.Sprintf("plugin config is not a valid object, using defaults: %v", err)}
	}

	var warnings []string
	if parsed.API.Port != nil {
		port := *parsed.API.Port
		if port >= 1 && port <= 65535 {
			cfg.APIPort = port
		} else {
			warnings = append(warnings, fmt.Sprintf("api.port %d is outside 1-65535, using %d", port, defaultAPIPort))
		}
	}
	cfg.APIToken = ResolveEnv(parsed.API.Token)
	return cfg, warnings
}

// ResolveEnv returns the value of environment variable X for "env.X", and the
// value unchanged otherwise.
func ResolveEnv(value string) string {
	if name, ok := strings.CutPrefix(value, "env."); ok {
		return os.Getenv(name)
	}
	return value
}
