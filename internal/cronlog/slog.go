// Package cronlog provides an adapter that implements github.com/robfig/cron/v3's Logger
// using the standard library's slog package.
package cronlog

import (
	"log/slog"

	"github.com/robfig/cron/v3"
)

type SlogLogger struct{}

var _ cron.Logger = (*SlogLogger)(nil)

func NewSlogLogger() *SlogLogger {
	return &SlogLogger{}
}

func (l *SlogLogger) Info(msg string, keysAndValues ...any) {
	slog.Info("cron: "+msg, keysAndValues...)
}

func (l *SlogLogger) Error(err error, msg string, keysAndValues ...any) {
	slog.Error("cron: "+msg, append(keysAndValues, []any{"error", err})...)
}
