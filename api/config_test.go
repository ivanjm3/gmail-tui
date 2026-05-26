package api

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := defaultConfig()
	if *cfg != *want {
		t.Fatalf("LoadConfig() = %+v, want %+v", cfg, want)
	}
}

func TestLoadConfigTOMLOverride(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	writeConfigFile(t, home, strings.Join([]string{
		"max_results = 25",
		"search_max_results = 50",
		"downloads_dir = \"custom-downloads\"",
		"max_concurrent = 8",
		"cache_max_size = 700",
		"log_level = \"debug\"",
	}, "\n"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.MaxResults != 25 || cfg.SearchMaxResults != 50 || cfg.DownloadsDir != "custom-downloads" {
		t.Fatalf("unexpected TOML overrides: %+v", cfg)
	}
	if cfg.MaxConcurrent != 8 || cfg.CacheMaxSize != 700 || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected TOML overrides: %+v", cfg)
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	writeConfigFile(t, home, "max_results = 25\ncache_max_size = 700\n")

	t.Setenv("GMAIL_TUI_MAX_RESULTS", "42")
	t.Setenv("GMAIL_TUI_SEARCH_MAX_RESULTS", "84")
	t.Setenv("GMAIL_TUI_DOWNLOADS_DIR", "env-downloads")
	t.Setenv("GMAIL_TUI_MAX_CONCURRENT", "12")
	t.Setenv("GMAIL_TUI_CACHE_MAX_SIZE", "900")
	t.Setenv("GMAIL_TUI_LOG_LEVEL", "warn")
	t.Setenv("GMAIL_TUI_CREDENTIALS", "env-credentials.json")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.MaxResults != 42 || cfg.SearchMaxResults != 84 {
		t.Fatalf("env overrides did not win: %+v", cfg)
	}
	if cfg.DownloadsDir != "env-downloads" || cfg.MaxConcurrent != 12 || cfg.CacheMaxSize != 900 {
		t.Fatalf("env overrides did not win: %+v", cfg)
	}
	if cfg.LogLevel != "WARN" || cfg.CredentialsPath != "env-credentials.json" {
		t.Fatalf("env overrides did not win: %+v", cfg)
	}
}

func TestLoadConfigUnknownKeyWarning(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	writeConfigFile(t, home, "max_results = 20\nunknown_key = \"value\"\n")

	logs := captureSlog(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.MaxResults != 20 {
		t.Fatalf("expected valid keys to load, got %+v", cfg)
	}
	if !strings.Contains(logs.String(), "unknown config key") || !strings.Contains(logs.String(), "unknown_key") {
		t.Fatalf("expected unknown key warning, got %q", logs.String())
	}
}

func TestConfigValidateClampsRanges(t *testing.T) {
	cfg := &Config{
		MaxResults:       0,
		SearchMaxResults: 9999,
		DownloadsDir:     "downloads",
		MaxConcurrent:    50,
		CacheMaxSize:     1,
		LogLevel:         "INFO",
	}

	logs := captureSlog(t)
	cfg.validate()

	if cfg.MaxResults != 1 || cfg.SearchMaxResults != 500 || cfg.MaxConcurrent != 20 || cfg.CacheMaxSize != 10 {
		t.Fatalf("validate() did not clamp values: %+v", cfg)
	}

	for _, field := range []string{"max_results", "search_max_results", "max_concurrent", "cache_max_size"} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("expected warning for %s in %q", field, logs.String())
		}
	}
}

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

func writeConfigFile(t *testing.T, home, contents string) {
	t.Helper()

	path := filepath.Join(home, ".config", "gmail-tui", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
	return &buf
}
