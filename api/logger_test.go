package api

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLoggerWritesJSONLogs(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "app.log")

	logger, err := NewLogger(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	t.Cleanup(func() {
		_ = logger.Close()
	})

	logger.Debug("debug message", "key", "value")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, want := range []string{"debug message", "info message", "warn message", "error message"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("expected log file to contain %q, got %q", want, string(data))
		}
	}
}

func TestNewLoggerFallsBackToNoop(t *testing.T) {
	base := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(base, []byte("file"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	logger, err := NewLogger(filepath.Join(base, "app.log"), slog.LevelInfo)
	if err == nil {
		t.Fatal("expected NewLogger() to return an error for invalid log path")
	}
	if logger == nil || logger.file != nil {
		t.Fatalf("expected noop logger fallback, got %+v", logger)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"debug": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
		"INFO":  slog.LevelInfo,
		"bad":   slog.LevelInfo,
	}

	for input, want := range tests {
		if got := ParseLogLevel(input); got != want {
			t.Fatalf("ParseLogLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestNoopLoggerClose(t *testing.T) {
	if err := NoopLogger().Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
