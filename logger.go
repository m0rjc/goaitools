package goaitools

import (
	"context"
	"log/slog"
)

// SlogSystemLogger is a SystemLogger implementation that logs to log/slog.
// It uses the default slog logger, which can be configured globally via slog.SetDefault().
type SlogSystemLogger struct{}

// NewSlogSystemLogger creates a new SystemLogger that logs to log/slog.
// This is the recommended logger for most applications.
//
// Example:
//
//	chat := &goaitools.Chat{
//	    Backend:      client,
//	    SystemLogger: goaitools.NewSlogSystemLogger(),
//	}
func NewSlogSystemLogger() SystemLogger {
	return SlogSystemLogger{}
}

func (s SlogSystemLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	slog.DebugContext(ctx, msg, keysAndValues...)
}

func (s SlogSystemLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	slog.InfoContext(ctx, msg, keysAndValues...)
}

func (s SlogSystemLogger) Error(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	if err != nil {
		keysAndValues = append(keysAndValues, "error", err)
	}
	slog.ErrorContext(ctx, msg, keysAndValues...)
}

// SilentLogger is a SystemLogger that does nothing.
// Use this when you want to disable all system logging.
type SilentLogger struct{}

// NewSilentLogger creates a SystemLogger that discards all log messages.
//
// Example:
//
//	chat := &goaitools.Chat{
//	    Backend:      client,
//	    SystemLogger: goaitools.NewSilentLogger(),
//	}
func NewSilentLogger() SystemLogger {
	return SilentLogger{}
}

func (s SilentLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {}
func (s SilentLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{})  {}
func (s SilentLogger) Error(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
}
