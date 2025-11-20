package core

import (
	"strings"

	"go.temporal.io/sdk/log"
)

// FilteredLogger wraps a Temporal logger and filters out specific warning messages
type FilteredLogger struct {
	logger log.Logger
}

// NewFilteredLogger creates a new filtered logger that suppresses HTTP 204 warnings
func NewFilteredLogger(logger log.Logger) *FilteredLogger {
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
func (f *FilteredLogger) With(keyvals ...interface{}) log.Logger {
	return &FilteredLogger{logger: log.With(f.logger, keyvals...)}
}
