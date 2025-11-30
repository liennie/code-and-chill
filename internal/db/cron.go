package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cc/internal/ctxlog"

	"github.com/robfig/cron/v3"
)

type backupJob struct {
	*DB

	backupDir  string
	backupName string
	backupNum  int
}

var _ cron.Job = (*backupJob)(nil)

func (j *backupJob) run() (string, error) {
	err := os.MkdirAll(j.backupDir, 0755)
	if err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	backupPath := filepath.Join(
		j.backupDir,
		j.backupName+"-"+time.Now().Format("2006-01-02-15-04-05.bak"),
	)
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer ctxlog.CloseErr(context.Background(), "backup file", backupFile)

	err = j.View(func(tx *Tx) error {
		_, err := tx.tx.WriteTo(backupFile)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("write backup file: %w", err)
	}

	entries, err := os.ReadDir(j.backupDir)
	if err != nil {
		return "", fmt.Errorf("read backup dir: %w", err)
	}

	prefix := j.backupName + "-"
	var backups []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".bak") {
			backups = append(backups, e)
		}
	}

	if len(backups) <= j.backupNum {
		return backupPath, nil
	}

	slices.SortFunc(backups, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, e := range backups[:len(backups)-j.backupNum] {
		p := filepath.Join(j.backupDir, e.Name())
		slog.Info("removing old db backup", "path", p)
		err = os.Remove(p)
		if err != nil {
			return "", fmt.Errorf("remove %q: %w", p, err)
		}
	}

	return backupPath, nil
}

func (j *backupJob) Run() {
	slog.Info("backing up db")

	now := time.Now()
	path, err := j.run()
	dur := time.Since(now)

	if err != nil {
		slog.Error("db backup failed", "error", err, "duration", dur.String())
	} else {
		slog.Info("db backed up", "path", path, "duration", dur.String())
	}
}
