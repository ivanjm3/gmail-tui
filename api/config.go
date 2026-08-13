package api

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration values.
// Values are loaded in order: defaults → TOML file → environment variable overrides.
type Config struct {
	MaxResults       int    `toml:"max_results"`        // default: 10
	SearchMaxResults int    `toml:"search_max_results"` // default: 30
	DownloadsDir     string `toml:"downloads_dir"`      // default: "downloads"
	MaxConcurrent    int    `toml:"max_concurrent"`     // default: 5, range [1,20]
	CacheMaxSize     int    `toml:"cache_max_size"`     // default: 500
	LogLevel         string `toml:"log_level"`          // default: "INFO"
	InboxQuery       string `toml:"inbox_query"`        // default: "in:inbox"
	CredentialsPath  string `toml:"-"`                  // from env GMAIL_TUI_CREDENTIALS
}

// defaultConfig returns a Config populated with all default values.
func defaultConfig() *Config {
	return &Config{
		MaxResults:       10,
		SearchMaxResults: 30,
		DownloadsDir:     "downloads",
		MaxConcurrent:    5,
		CacheMaxSize:     500,
		LogLevel:         "INFO",
		InboxQuery:       "in:inbox",
	}
}

// LoadConfig loads configuration using the following precedence (lowest to highest):
//  1. Built-in defaults
//  2. TOML file at ~/.config/gmail-tui/config.toml (if it exists)
//  3. Environment variable overrides (GMAIL_TUI_* prefix)
func LoadConfig() (*Config, error) {
	cfg := defaultConfig()

	// Load from TOML file if it exists.
	tomlPath, err := configFilePath()
	if err == nil {
		if _, statErr := os.Stat(tomlPath); statErr == nil {
			// File exists — parse it.
			meta, decodeErr := toml.DecodeFile(tomlPath, cfg)
			if decodeErr != nil {
				return nil, decodeErr
			}
			// Warn about unknown keys.
			for _, key := range meta.Undecoded() {
				slog.Warn("unknown config key", "key", key.String())
			}
		}
		// If the file doesn't exist, silently use defaults.
	}

	// Apply environment variable overrides.
	cfg.applyEnvOverrides()

	// Validate and clamp out-of-range values.
	cfg.validate()

	return cfg, nil
}

// configFilePath returns the path to the TOML config file.
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gmail-tui", "config.toml"), nil
}

// applyEnvOverrides reads GMAIL_TUI_* environment variables and overrides
// the corresponding Config fields when the variable is set.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("GMAIL_TUI_MAX_RESULTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxResults = n
		}
	}
	if v := os.Getenv("GMAIL_TUI_SEARCH_MAX_RESULTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.SearchMaxResults = n
		}
	}
	if v := os.Getenv("GMAIL_TUI_DOWNLOADS_DIR"); v != "" {
		c.DownloadsDir = v
	}
	if v := os.Getenv("GMAIL_TUI_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxConcurrent = n
		}
	}
	if v := os.Getenv("GMAIL_TUI_CACHE_MAX_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.CacheMaxSize = n
		}
	}
	if v := os.Getenv("GMAIL_TUI_LOG_LEVEL"); v != "" {
		c.LogLevel = strings.ToUpper(v)
	}
	if v := os.Getenv("GMAIL_TUI_INBOX_QUERY"); v != "" {
		c.InboxQuery = v
	}
	if v := os.Getenv("GMAIL_TUI_CREDENTIALS"); v != "" {
		c.CredentialsPath = v
	}
}

// validate clamps out-of-range values and logs a warning for each clamped field.
func (c *Config) validate() {
	if c.MaxConcurrent < 1 || c.MaxConcurrent > 20 {
		slog.Warn("max_concurrent out of range [1,20], clamping",
			"value", c.MaxConcurrent,
			"default", 5,
		)
		c.MaxConcurrent = clamp(c.MaxConcurrent, 1, 20)
	}
	if c.MaxResults < 1 || c.MaxResults > 500 {
		slog.Warn("max_results out of range [1,500], clamping",
			"value", c.MaxResults,
			"default", 10,
		)
		c.MaxResults = clamp(c.MaxResults, 1, 500)
	}
	if c.SearchMaxResults < 1 || c.SearchMaxResults > 500 {
		slog.Warn("search_max_results out of range [1,500], clamping",
			"value", c.SearchMaxResults,
			"default", 30,
		)
		c.SearchMaxResults = clamp(c.SearchMaxResults, 1, 500)
	}
	if strings.TrimSpace(c.InboxQuery) == "" {
		c.InboxQuery = "in:inbox"
	}
	if c.CacheMaxSize < 10 || c.CacheMaxSize > 10000 {
		slog.Warn("cache_max_size out of range [10,10000], clamping",
			"value", c.CacheMaxSize,
			"default", 500,
		)
		c.CacheMaxSize = clamp(c.CacheMaxSize, 10, 10000)
	}
}

// clamp returns v clamped to the inclusive range [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
