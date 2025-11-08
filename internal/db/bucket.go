package db

import (
	"bytes"
	"encoding/gob"
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
	// this should never panic unless we try to gob unsuported types
	must(gob.NewDecoder(bytes.NewReader(data)).Decode(val))
	return val
}

func (b *Bucket[V]) Get(key string) *V {
	return b.decode(b.b.Get([]byte(key)))
}

func (b *Bucket[V]) Put(key string, val *V) error {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(val)
	if err != nil {
		// this should never happen
		return fmt.Errorf("marshal: %w", err)
	}

	return b.b.Put([]byte(key), buf.Bytes())
}

func (b *Bucket[V]) Delete(key string) error {
	return b.b.Delete([]byte(key))
}

var errStop = fmt.Errorf("stop iteration")

func (b *Bucket[V]) All() iter.Seq2[string, *V] {
	return func(yield func(string, *V) bool) {
		b.b.ForEach(func(k, v []byte) error {
			val := b.decode(v)
			if !yield(string(k), val) {
				return errStop
			}
			return nil
		})
	}
}
