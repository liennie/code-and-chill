package db

import (
	"cc/internal/ctxlog"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
)

type backupJob struct {
	*DB

	backupDir  string
	backupName string
}

var _ cron.Job = (*backupJob)(nil)

func (j *backupJob) run() error {
	err := os.MkdirAll(j.backupDir, 0755)
	if err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	backupFile, err := os.Create(filepath.Join(
		j.backupDir,
		j.backupName+"-"+time.Now().Format("2006-01-02-15-04-05.bak"),
	))
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer ctxlog.CloseErr(context.Background(), "backup file", backupFile)

	return j.View(func(tx *Tx) error {
		_, err := tx.tx.WriteTo(backupFile)
		return err
	})
}

func (j *backupJob) Run() {
	slog.Info("backing up db")

	now := time.Now()
	err := j.run()
	dur := time.Since(now)

	if err != nil {
		slog.Error("backup failed", "error", err, "duration", dur.String())
	} else {
		slog.Info("backup", "duration", dur.String())
	}
}
