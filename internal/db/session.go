package db

import (
	"time"
)

var (
	bucketSession       = []byte("session")
	bucketSessionExpire = []byte("session_expire")
)

type Session struct {
	ID     string
	Expire time.Time
}

func (tx *Tx) Session() *Bucket[Session] {
	return openBucket[Session](tx.tx, bucketSession)
}

type SessionExpire struct {
	IDs []string
}

func (tx *Tx) SessionExpire() *Bucket[SessionExpire] {
	return openBucket[SessionExpire](tx.tx, bucketSessionExpire)
}
