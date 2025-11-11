package db

import (
	"fmt"
	"log/slog"

	"go.etcd.io/bbolt"
)

type slogLogger struct{}

var _ bbolt.Logger = (*slogLogger)(nil)

func (l *slogLogger) Debug(v ...interface{}) {
	slog.Debug("bolt: " + fmt.Sprint(v...))
}

func (l *slogLogger) Debugf(format string, v ...interface{}) {
	slog.Debug("bolt: " + fmt.Sprintf(format, v...))
}

func (l *slogLogger) Error(v ...interface{}) {
	slog.Error("bolt: " + fmt.Sprint(v...))
}

func (l *slogLogger) Errorf(format string, v ...interface{}) {
	slog.Error("bolt: " + fmt.Sprintf(format, v...))
}

func (l *slogLogger) Info(v ...interface{}) {
	slog.Info("bolt: " + fmt.Sprint(v...))
}

func (l *slogLogger) Infof(format string, v ...interface{}) {
	slog.Info("bolt: " + fmt.Sprintf(format, v...))
}

func (l *slogLogger) Warning(v ...interface{}) {
	slog.Warn("bolt: " + fmt.Sprint(v...))
}

func (l *slogLogger) Warningf(format string, v ...interface{}) {
	slog.Warn("bolt: " + fmt.Sprintf(format, v...))
}

func (l *slogLogger) Fatal(v ...interface{}) {
	msg := "bolt: " + fmt.Sprint(v...)
	slog.Error(msg)
	panic(msg)
}

func (l *slogLogger) Fatalf(format string, v ...interface{}) {
	msg := "bolt: " + fmt.Sprintf(format, v...)
	slog.Error(msg)
	panic(msg)
}

func (l *slogLogger) Panic(v ...interface{}) {
	msg := "bolt: " + fmt.Sprint(v...)
	slog.Error(msg)
	panic(msg)
}

func (l *slogLogger) Panicf(format string, v ...interface{}) {
	msg := "bolt: " + fmt.Sprintf(format, v...)
	slog.Error(msg)
	panic(msg)
}
