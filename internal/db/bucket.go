package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"

	"go.etcd.io/bbolt"
)

var (
	BucketSession       = []byte("session") // TODO unite session and session_data
	BucketSessionExpire = []byte("session_expire")
	BucketSessionData   = []byte("session_data")

	BucketUser        = []byte("user")
	BucketDiscordUser = []byte("discord_user")

	BucketProgress = []byte("progress")
)

var allBuckets = [][]byte{
	BucketSession,
	BucketSessionExpire,
	BucketSessionData,

	BucketUser,
	BucketDiscordUser,

	BucketProgress,
}

type BucketKey[V any] struct {
	key []byte
}

func NewBucketKey[V any](db *DB, key []byte) *BucketKey[V] {
	err := db.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(key)
		if b == nil {
			return fmt.Errorf("db: bucket %q is missing", key)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return &BucketKey[V]{key: key}
}

func (f *BucketKey[V]) Open(tx *Tx) *Bucket[V] {
	b := tx.tx.Bucket(f.key)
	if b == nil {
		// this should never happen
		panic(fmt.Errorf("db: bucket %q is missing", f.key))
	}
	return &Bucket[V]{b: b, tx: tx}
}

type Bucket[V any] struct {
	b  *bbolt.Bucket
	tx *Tx
}

func (b *Bucket[V]) Has(key string) bool {
	return b.b.Get([]byte(key)) != nil
}

type KeySetter interface {
	SetKey(string)
}

func (b *Bucket[V]) decode(key string, data []byte) *V {
	if data == nil {
		return nil
	}

	val := new(V)
	// this should never panic unless we try to json unsuported types
	must(json.NewDecoder(bytes.NewReader(data)).Decode(val))

	if ks, ok := any(val).(KeySetter); ok {
		ks.SetKey(key)
	}

	return val
}

func (b *Bucket[V]) Get(key string) *V {
	return b.decode(key, b.b.Get([]byte(key)))
}

func (b *Bucket[V]) Put(key string, val *V) error {
	data, err := json.Marshal(val)
	if err != nil {
		// this should never happen
		return fmt.Errorf("marshal: %w", err)
	}

	err = b.b.Put([]byte(key), data)
	if err != nil {
		return err
	}

	b.tx.modified = true
	return nil
}

func (b *Bucket[V]) Delete(key string) error {
	err := b.b.Delete([]byte(key))
	if err != nil {
		return err
	}

	b.tx.modified = true
	return nil
}

func (b *Bucket[V]) NextSequence() (uint64, error) {
	seq, err := b.b.NextSequence()
	if err != nil {
		return seq, err
	}

	b.tx.modified = true
	return seq, nil
}

func (b *Bucket[V]) HasBucket(key string) bool {
	return b.b.Bucket([]byte(key)) != nil
}

func (b *Bucket[V]) Bucket(key string) *Bucket[V] {
	sub := b.b.Bucket([]byte(key))
	if sub == nil {
		return nil
	}
	return &Bucket[V]{b: sub, tx: b.tx}
}

func (b *Bucket[V]) CreateBucket(key string) (*Bucket[V], error) {
	sub, err := b.b.CreateBucketIfNotExists([]byte(key))
	if err != nil {
		return nil, err
	}

	b.tx.modified = true
	return &Bucket[V]{b: sub, tx: b.tx}, nil
}

func (b *Bucket[V]) DeleteBucket(key string) error {
	err := b.b.DeleteBucket([]byte(key))
	if err != nil {
		return err
	}

	b.tx.modified = true
	return nil
}

type startFunc func(c *bbolt.Cursor) (k, v []byte)
type contFunc func(k []byte) bool

func rangeFuncs(from, to string) (start startFunc, cont contFunc) {
	if from == "" {
		start = func(c *bbolt.Cursor) (k, v []byte) {
			return c.First()
		}
	} else {
		start = func(c *bbolt.Cursor) (k, v []byte) {
			return c.Seek([]byte(from))
		}
	}

	if to == "" {
		cont = func(k []byte) bool {
			return k != nil
		}
	} else {
		max := []byte(to)
		cont = func(k []byte) bool {
			return k != nil && bytes.Compare(k, max) <= 0
		}
	}
	return
}

func (b *Bucket[V]) Range(from, to string) iter.Seq2[string, *V] {
	start, cont := rangeFuncs(from, to)
	return func(yield func(string, *V) bool) {
		c := b.b.Cursor()
		for k, v := start(c); cont(k); k, v = c.Next() {
			if v == nil {
				continue
			}

			key := string(k)
			val := b.decode(key, v)
			if !yield(key, val) {
				break
			}
		}
	}
}

func (b *Bucket[V]) All() iter.Seq2[string, *V] {
	return b.Range("", "")
}

func (b *Bucket[V]) DeleteRange(from, to string) error {
	start, cont := rangeFuncs(from, to)
	c := b.b.Cursor()
	for k, v := start(c); cont(k); k, v = c.Next() {
		if v == nil {
			continue
		}

		err := c.Delete()
		if err != nil {
			return err
		}
	}
	b.tx.modified = true
	return nil
}
