package sched

import (
	"log/slog"

	"github.com/robfig/cron/v3"
)

type slogLogger struct{}

var _ cron.Logger = (*slogLogger)(nil)

func (l *slogLogger) Info(msg string, keysAndValues ...any) {
	slog.Info("cron "+msg, keysAndValues...)
}

func (l *slogLogger) Error(err error, msg string, keysAndValues ...any) {
	slog.Error("cron "+msg, append(keysAndValues, "error", err)...)
}
