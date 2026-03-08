// Package db provides functions for managing db data using a BoltDB backend.
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

	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/sched"

	"github.com/robfig/cron/v3"
	"go.etcd.io/bbolt"
)

type DB struct {
	db       *bbolt.DB
	closeErr error

	backupDir  string
	backupName string
	backupNum  int

	backupJobID *cron.EntryID
}

func Open(config Config) *DB {
	if config.File == "" {
		panic("db: file is required")
	}

	err := os.MkdirAll(filepath.Dir(config.File), 0755)
	if err != nil {
		panic(fmt.Errorf("db: create db dir: %w", err))
	}

	db, err := bbolt.Open(config.File, 0600, &bbolt.Options{
		Timeout: 30 * time.Second,
		Logger:  &slogLogger{},
	})
	if err != nil {
		panic(fmt.Errorf("db: open bbolt db: %w", err))
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range allBuckets {
			_, err := tx.CreateBucketIfNotExists(bucket)
			if err != nil {
				return fmt.Errorf("create bucket %q: %w", bucket, err)
			}
		}

		return nil
	})
	if err != nil {
		ctxlog.CloseErr(context.Background(), "db", db)
		panic(fmt.Errorf("db: initialize buckets: %w", err))
	}

	d := &DB{db: db}

	if config.BackupSchedule != "" {
		if config.BackupDir == "" {
			panic("db: backupDir is required if backupSchedule is set")
		}
		if config.BackupNum == 0 {
			config.BackupNum = 84
		}

		d.backupDir = config.BackupDir
		d.backupName = filepath.Base(config.File)
		d.backupNum = config.BackupNum

		schedule, err := cron.ParseStandard(config.BackupSchedule)
		if err != nil {
			ctxlog.CloseErr(context.Background(), "db", db)
			panic(fmt.Errorf("db: backupSchedule: %w", err))
		}
		backupJobID := sched.Cron.Schedule(schedule, &backupJob{
			DB: d,
		})
		d.backupJobID = &backupJobID
	}

	return d
}

func (db *DB) Close() error {
	if db.backupJobID != nil {
		sched.Cron.Remove(*db.backupJobID)
		db.backupJobID = nil
	}

	if db.db == nil {
		return db.closeErr
	}

	err := db.db.Close()
	db.db = nil

	if err != nil {
		db.closeErr = fmt.Errorf("db: close bbolt db: %w", err)
	}
	return db.closeErr
}

func must(err error) {
	if err != nil {
		panic(fmt.Errorf("db: must: %w", err))
	}
}

func (db *DB) backup() (string, error) {
	err := os.MkdirAll(db.backupDir, 0755)
	if err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	backupPath := filepath.Join(
		db.backupDir,
		db.backupName+"-"+time.Now().Format("2006-01-02-15-04-05.bak"),
	)
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer ctxlog.CloseErr(context.Background(), "backup file", backupFile)

	err = db.View(func(tx *Tx) error {
		_, err := tx.tx.WriteTo(backupFile)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("write backup file: %w", err)
	}

	entries, err := os.ReadDir(db.backupDir)
	if err != nil {
		return "", fmt.Errorf("read backup dir: %w", err)
	}

	prefix := db.backupName + "-"
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

	if len(backups) <= db.backupNum {
		return backupPath, nil
	}

	slices.SortFunc(backups, func(a, b os.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, e := range backups[:len(backups)-db.backupNum] {
		p := filepath.Join(db.backupDir, e.Name())
		slog.Info("removing old db backup", "path", p)
		err = os.Remove(p)
		if err != nil {
			return "", fmt.Errorf("remove %q: %w", p, err)
		}
	}

	return backupPath, nil
}

func (db *DB) Backup() {
	slog.Info("backing up db")

	now := time.Now()
	path, err := db.backup()
	dur := time.Since(now)

	if err != nil {
		slog.Error("db backup failed", "error", err, "duration", dur.String())
	} else {
		slog.Info("db backed up", "path", path, "duration", dur.String())
	}
}
