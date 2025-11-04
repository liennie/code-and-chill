package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"iter"

	"go.etcd.io/bbolt"
)

type bucket[V any] struct {
	b *bbolt.Bucket
}

func openBucket[V any](tx *bbolt.Tx, key []byte) *bucket[V] {
	b := tx.Bucket(key)
	if b == nil {
		// this should never happen
		panic(fmt.Errorf("db: bucket %q is missing", key))
	}
	return &bucket[V]{b: b}
}

func (b *bucket[V]) has(key []byte) bool {
	return b.b.Get(key) != nil
}

func (b *bucket[V]) decode(data []byte) *V {
	if data == nil {
		return nil
	}

	val := new(V)
	// this should never panic unless we try to gob unsuported types
	must(gob.NewDecoder(bytes.NewReader(data)).Decode(val))
	return val
}

func (b *bucket[V]) get(key []byte) *V {
	return b.decode(b.b.Get(key))
}

func (b *bucket[V]) put(key []byte, val *V) error {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(val)
	if err != nil {
		// this should never happen
		return fmt.Errorf("marshal: %w", err)
	}

	return b.b.Put(key, buf.Bytes())
}

func (b *bucket[V]) delete(key []byte) error {
	return b.b.Delete(key)
}

var errStop = fmt.Errorf("stop iteration")

func (b *bucket[V]) all() iter.Seq2[[]byte, *V] {
	return func(yield func([]byte, *V) bool) {
		b.b.ForEach(func(k, v []byte) error {
			val := b.decode(v)
			if !yield(k, val) {
				return errStop
			}
			return nil
		})
	}
}
