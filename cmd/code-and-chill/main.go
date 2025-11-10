package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cc/internal/auth"
	"cc/internal/ctxlog"
	"cc/internal/db"
	"cc/internal/puzzles"
	"cc/internal/rec"
	"cc/internal/server"
	"cc/internal/session"
)

func run(ctx context.Context, config string) (err error) {
	defer rec.Error(&err)

	logger := ctxlog.Get(ctx)

	c, err := LoadConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger.Info("opening db")
	db := db.Open(c.DB)
	defer ctxlog.Close(ctx, "db", db)

	logger.Info("starting session store")
	sess := session.NewStore(c.Session, db)
	defer ctxlog.Close(ctx, "session store", sess)

	logger.Info("starting auth")
	auth := auth.New(c.Auth, db)

	logger.Info("loading puzzles")
	puzzles := puzzles.Load(c.Puzzles)

	logger.Info("starting server")
	srv := server.New(c.Server, db, sess, auth, puzzles)

	return srv.Run(ctx)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ctx = ctxlog.Setup(ctx, "server")

	logger := ctxlog.Get(ctx)

	config := "config.yaml"
	if len(os.Args) > 1 {
		config = os.Args[1]
	}

	err := run(ctx, config)
	if err != nil {
		logger.Error("server stopped unexpectedly", "error", err)
	} else {
		logger.Info("server gracefully stopped")
	}
}
