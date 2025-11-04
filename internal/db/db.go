// Package db provides functions for managing db data using a BoltDB backend.
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var allBuckets = [][]byte{
	bucketSession,
	bucketSessionExpire,
}

type DB struct {
	db       *bbolt.DB
	closeErr error
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

	return &DB{db: db}
}

func (db *DB) Close() error {
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
