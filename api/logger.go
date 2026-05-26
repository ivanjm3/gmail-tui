package api

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Logger wraps log/slog for structured, leveled, file-based logging.
// All output goes to a file; nothing is written to stdout/stderr during TUI operation.
type Logger struct {
	handler slog.Handler
	file    *os.File // nil for no-op logger
	logger  *slog.Logger
}

// NewLogger creates a Logger that writes JSON-structured log entries to logPath.
// The directory is created if it does not exist.
// If the file cannot be opened, returns a no-op Logger and a non-nil error.
func NewLogger(logPath string, level slog.Level) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		noop := NoopLogger()
		return noop, fmt.Errorf("logger: failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		noop := NoopLogger()
		return noop, fmt.Errorf("logger: failed to open log file: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	l := &Logger{
		handler: handler,
		file:    f,
		logger:  slog.New(handler),
	}
	return l, nil
}

// NoopLogger returns a Logger that discards all output.
func NoopLogger() *Logger {
	handler := slog.NewJSONHandler(io.Discard, nil)
	return &Logger{
		handler: handler,
		file:    nil,
		logger:  slog.New(handler),
	}
}

// ParseLogLevel converts a string like "DEBUG", "INFO", "WARN", "ERROR" to slog.Level.
// Returns slog.LevelInfo for unrecognized strings.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Close flushes and closes the underlying log file.
// Returns nil if the logger is a no-op (no file to close).
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) { l.logger.Info(msg, args...) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) { l.logger.Warn(msg, args...) }

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) { l.logger.Error(msg, args...) }

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) { l.logger.Debug(msg, args...) }
