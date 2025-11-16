package db

import (
	"errors"

	"go.etcd.io/bbolt"
)

type Tx struct {
	tx       *bbolt.Tx
	modified bool
}

func newTx(tx *bbolt.Tx) *Tx {
	return &Tx{tx: tx}
}

func (db *DB) View(fn func(tx *Tx) error) error {
	return db.db.View(func(tx *bbolt.Tx) error {
		return fn(newTx(tx))
	})
}

var errRollback = errors.New("rollback")

func (db *DB) Update(fn func(tx *Tx) error) error {
	err := db.db.Update(func(btx *bbolt.Tx) error {
		tx := newTx(btx)
		err := fn(tx)
		if err != nil {
			return err
		}
		if !tx.modified {
			return errRollback
		}
		return nil
	})
	if errors.Is(err, errRollback) {
		return nil
	}
	return err
}
