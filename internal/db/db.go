// Package db provides functions for managing db data using a BoltDB backend.
package db

import (
	"cc/internal/sched"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
	"go.etcd.io/bbolt"
)

type DB struct {
	db       *bbolt.DB
	closeErr error

	backupJobID *cron.EntryID
}

func Open(config Config) *DB {
	if config.File == "" {
		panic("db: file is required")
	}
	if config.BackupSchedule == "" {
		panic("db: backupSchedule is required")
	}
	if config.BackupDir == "" {
		panic("db: backupDir is required")
	}

	err := os.MkdirAll(filepath.Dir(config.File), 0755)
	if err != nil {
		panic(fmt.Errorf("db: create db dir: %w", err))
	}

	schedule, err := cron.ParseStandard(config.BackupSchedule)
	if err != nil {
		panic(fmt.Errorf("db: backupSchedule: %w", err))
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
		db.Close()
		panic(fmt.Errorf("db: initialize buckets: %w", err))
	}

	d := &DB{db: db}
	backupJobID := sched.Cron.Schedule(schedule, &backupJob{
		DB:         d,
		backupDir:  config.BackupDir,
		backupName: filepath.Base(config.File),
	})
	d.backupJobID = &backupJobID

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
