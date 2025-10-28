// Package db provides functions for managing db data using a BoltDB backend.
package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var (
	bucketTeams = []byte("teams")
)

var allBuckets = [][]byte{
	bucketTeams,
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

func get[T *V, V any](b *bbolt.Bucket, key string) (T, error) {
	val := new(V)

	data := b.Get([]byte(key))
	if data != nil {
		err := gob.NewDecoder(bytes.NewReader(data)).Decode(val)
		if err != nil {
			return nil, fmt.Errorf("unmarshal value for %q: %w", key, err)
		}
	}

	return val, nil
}

var errStop = fmt.Errorf("stop iteration")

func all[T *V, V any](db *DB, bucket string) iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		err := db.db.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(bucket))
			if b == nil {
				return fmt.Errorf("db: bucket %q not found", bucket)
			}

			return b.ForEach(func(k, v []byte) error {
				val, err := get[T](b, string(k))
				if err != nil {
					return fmt.Errorf("db: get value for %q: %w", k, err)
				}

				if !yield(string(k), val) {
					return errStop
				}
				return nil
			})
		})

		if err != nil {
			if errors.Is(err, errStop) {
				return
			}
			panic(fmt.Errorf("db: get all from bucket %q: %w", bucket, err))
		}
	}
}
