package db

import (
	"time"
)

var (
	bucketSession       = []byte("session")
	bucketSessionExpire = []byte("session_expire")
	bucketSessionData   = []byte("session_data")
)

type Session struct {
	Expire time.Time
}

func (tx *Tx) Session() *Bucket[Session] {
	return openBucket[Session](tx, bucketSession)
}

type SessionExpire struct {
	IDs []string
}

func (tx *Tx) SessionExpire() *Bucket[SessionExpire] {
	return openBucket[SessionExpire](tx, bucketSessionExpire)
}

func SessionData[T any](tx *Tx) *Bucket[T] {
	return openBucket[T](tx, bucketSessionData)
}
