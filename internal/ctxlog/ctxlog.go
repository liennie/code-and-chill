// Package ctxlog provides context-aware structured logging utilities.
package ctxlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

var setup = false
var logFile io.WriteCloser

func Setup(ctx context.Context, name string) context.Context {
	if setup {
		return Store(ctx, slog.Default())
	}

	err := os.MkdirAll("log", 0755)
	if err != nil {
		panic(fmt.Errorf("create log dir: %w", err))
	}

	logFile, err = os.Create(filepath.Join("log", name+"-"+time.Now().Format("2006-01-02-15-04-05.log")))
	if err != nil {
		panic(fmt.Errorf("create log file: %w", err))
	}

	w := io.MultiWriter(os.Stderr, logFile)

	logger := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{ /*Level: slog.LevelDebug*/ }))
	slog.SetDefault(logger)

	setup = true

	return Store(ctx, logger)
}

func Close(ctx context.Context) error {
	if setup && logFile != nil {
		defer func() {
			setup = false
			logFile = nil
		}()

		return CloseErr(ctx, "log file", logFile)
	}
	return nil
}

type ctxKey struct{}

var key ctxKey

func Store(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, key, log)
}

func Get(ctx context.Context) *slog.Logger {
	log, ok := ctx.Value(key).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return log
}

func CloseErr(ctx context.Context, name string, closer io.Closer) error {
	logger := Get(ctx)
	err := closer.Close()
	if err != nil {
		logger.Error("failed to close", "closer", name, "error", err)
		return err
	}
	return nil
}

func With(ctx context.Context, kv ...any) context.Context {
	return Store(ctx, Get(ctx).With(kv...))
}

func ErrExtra(err error) (string, bool) {
	var extra interface {
		Extra() string
	}
	ok := errors.As(err, &extra)
	if !ok {
		return "", false
	}
	return extra.Extra(), true
}
