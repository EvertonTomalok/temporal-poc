package core

import (
	"context"
	"log/slog"
	"os"
	"strings"

	tlog "go.temporal.io/sdk/log"
)

// WarningFilter filters out warning-level log messages
type WarningFilter struct {
	handler slog.Handler
}

// Enabled returns whether the handler should process a log record at the given level
func (w *WarningFilter) Enabled(ctx context.Context, level slog.Level) bool {
	// Filter out warnings, allow info and errors
	if level == slog.LevelWarn {
		return false
	}
	return w.handler.Enabled(ctx, level)
}

// Handle processes a log record, skipping warnings
func (w *WarningFilter) Handle(ctx context.Context, record slog.Record) error {
	// Skip warnings
	if record.Level == slog.LevelWarn {
		return nil
	}
	return w.handler.Handle(ctx, record)
}

// WithAttrs returns a new handler with additional attributes
func (w *WarningFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &WarningFilter{handler: w.handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with a group name
func (w *WarningFilter) WithGroup(name string) slog.Handler {
	return &WarningFilter{handler: w.handler.WithGroup(name)}
}

// NewLoggerWithoutWarnings creates a Temporal logger that filters out all warnings
// but keeps info and error messages
func NewLoggerWithoutWarnings() tlog.Logger {
	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	warningFilterHandler := &WarningFilter{handler: baseHandler}
	return tlog.NewStructuredLogger(slog.New(warningFilterHandler))
}

// FilteredLogger wraps a Temporal logger and filters out specific warning messages
type FilteredLogger struct {
	logger tlog.Logger
}

// NewFilteredLogger creates a new filtered logger that suppresses HTTP 204 warnings
func NewFilteredLogger(logger tlog.Logger) *FilteredLogger {
	return &FilteredLogger{logger: logger}
}

// shouldFilter checks if a warning message should be filtered out
func (f *FilteredLogger) shouldFilter(msg string) bool {
	// Filter out the HTTP 204 warning about missing content-type header
	return strings.Contains(msg, "204 (No Content)") &&
		strings.Contains(msg, "malformed header: missing HTTP content-typ")
}

// Debug logs a debug message
func (f *FilteredLogger) Debug(msg string, keyvals ...interface{}) {
	f.logger.Debug(msg, keyvals...)
}

// Info logs an info message
func (f *FilteredLogger) Info(msg string, keyvals ...interface{}) {
	f.logger.Info(msg, keyvals...)
}

// Warn logs a warning message, but filters out the HTTP 204 warning
func (f *FilteredLogger) Warn(msg string, keyvals ...interface{}) {
	if f.shouldFilter(msg) {
		// Silently ignore this specific warning
		return
	}
	f.logger.Warn(msg, keyvals...)
}

// Error logs an error message
func (f *FilteredLogger) Error(msg string, keyvals ...interface{}) {
	f.logger.Error(msg, keyvals...)
}

// With returns a new logger with additional key-value pairs
func (f *FilteredLogger) With(keyvals ...interface{}) tlog.Logger {
	return &FilteredLogger{logger: tlog.With(f.logger, keyvals...)}
}
