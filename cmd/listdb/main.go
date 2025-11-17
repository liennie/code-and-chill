package main

import (
	"fmt"
	"os"
	"time"

	"go.etcd.io/bbolt"
)

func listBucket(name []byte, b *bbolt.Bucket, prefix string) error {
	seq := b.Sequence()
	fmt.Printf("%s%q: Bucket [seq=%d]\n", prefix, name, seq)
	prefix = prefix + "  "

	err := b.ForEach(func(k, v []byte) error {
		if v == nil {
			sub := b.Bucket(k)
			if sub == nil {
				fmt.Printf("%s%q: <nil bucket>\n", prefix, k)
			} else {
				if err := listBucket(k, sub, prefix+"  "); err != nil {
					return err
				}
			}
		} else {
			fmt.Printf("%s%q: %s\n", prefix, k, v)
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func main() {
	file := "db/cc.db"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	db, err := bbolt.Open(file, 0600, &bbolt.Options{
		Timeout:  30 * time.Second,
		ReadOnly: true,
	})
	if err != nil {
		panic(fmt.Errorf("db: open bbolt db: %w", err))
	}

	err = db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			return listBucket(name, b, "")
		})
	})
	if err != nil {
		panic(fmt.Errorf("db: view tx: %w", err))
	}
}
