package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"

	"go.etcd.io/bbolt"
)

type Bucket[V any] struct {
	b *bbolt.Bucket
}

func openBucket[V any](tx *bbolt.Tx, key []byte) *Bucket[V] {
	b := tx.Bucket(key)
	if b == nil {
		// this should never happen
		panic(fmt.Errorf("db: bucket %q is missing", key))
	}
	return &Bucket[V]{b: b}
}

func (b *Bucket[V]) Has(key string) bool {
	return b.b.Get([]byte(key)) != nil
}

func (b *Bucket[V]) decode(data []byte) *V {
	if data == nil {
		return nil
	}

	val := new(V)
	// this should never panic unless we try to json unsuported types
	must(json.NewDecoder(bytes.NewReader(data)).Decode(val))
	return val
}

func (b *Bucket[V]) Get(key string) *V {
	return b.decode(b.b.Get([]byte(key)))
}

func (b *Bucket[V]) Put(key string, val *V) error {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(val)
	if err != nil {
		// this should never happen
		return fmt.Errorf("marshal: %w", err)
	}

	return b.b.Put([]byte(key), buf.Bytes())
}

func (b *Bucket[V]) Delete(key string) error {
	return b.b.Delete([]byte(key))
}

func (b *Bucket[V]) NextSequence() (uint64, error) {
	return b.b.NextSequence()
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
			val := b.decode(v)
			if !yield(string(k), val) {
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
	for k, _ := start(c); cont(k); k, _ = c.Next() {
		err := c.Delete()
		if err != nil {
			return err
		}
	}
	return nil
}
