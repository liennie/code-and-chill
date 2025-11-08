package db

import (
	"go.etcd.io/bbolt"
)

type Tx struct {
	tx *bbolt.Tx
}

func newTx(tx *bbolt.Tx) *Tx {
	return &Tx{tx: tx}
}

func (db *DB) View(fn func(tx *Tx) error) error {
	return db.db.View(func(tx *bbolt.Tx) error {
		return fn(newTx(tx))
	})
}

func (db *DB) Update(fn func(tx *Tx) error) error {
	return db.db.Update(func(tx *bbolt.Tx) error {
		return fn(newTx(tx))
	})
}
