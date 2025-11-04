package db

import (
	"slices"
	"time"

	"go.etcd.io/bbolt"
)

var (
	bucketSession       = []byte("session")
	bucketSessionExpire = []byte("session_expire")
)

type Session struct {
	ID     string
	Expire time.Time
}

func (db *DB) session(tx *bbolt.Tx) *bucket[Session] {
	return openBucket[Session](tx, bucketSession)
}

type sessionExpire struct {
	IDs []string
}

func (db *DB) sessionExpire(tx *bbolt.Tx) *bucket[sessionExpire] {
	return openBucket[sessionExpire](tx, bucketSessionExpire)
}

func (db *DB) deleteSessionExpire(tx *bbolt.Tx, s *Session) error {
	bucket := db.sessionExpire(tx)

	expire := s.Expire.UTC().Format(time.RFC3339)

	sessions := bucket.get([]byte(expire))
	if sessions == nil {
		return nil
	}

	sessions.IDs = slices.DeleteFunc(sessions.IDs, func(id string) bool { return id == s.ID })
	if len(sessions.IDs) == 0 {
		return bucket.delete([]byte(expire))
	} else {
		return bucket.put([]byte(expire), sessions)
	}
}

func (db *DB) addSessionExpire(tx *bbolt.Tx, s *Session) error {
	bucket := db.sessionExpire(tx)

	expire := s.Expire.UTC().Format(time.RFC3339)

	sessions := bucket.get([]byte(expire))
	if sessions == nil {
		sessions = &sessionExpire{
			IDs: []string{s.ID},
		}
	} else {
		sessions.IDs = append(sessions.IDs, s.ID)
	}

	return bucket.put([]byte(expire), sessions)
}

func (db *DB) HasSession(id string) (has bool) {
	db.db.View(func(tx *bbolt.Tx) error {
		has = db.session(tx).has([]byte(id))
		return nil
	})
	return has
}

func (db *DB) GetSession(id string) (s *Session) {
	db.db.View(func(tx *bbolt.Tx) error {
		s = db.session(tx).get([]byte(id))
		return nil
	})
	return s
}

func (db *DB) PutSession(s *Session) error {
	return db.db.Update(func(tx *bbolt.Tx) error {
		bucket := db.session(tx)

		old := bucket.get([]byte(s.ID))
		if old != nil {
			if err := db.deleteSessionExpire(tx, old); err != nil {
				return err
			}
		}

		if err := db.addSessionExpire(tx, s); err != nil {
			return err
		}

		return bucket.put([]byte(s.ID), s)
	})
}

func (db *DB) DeleteSession(id string) error {
	return db.db.Update(func(tx *bbolt.Tx) error {
		old := db.session(tx).get([]byte(id))
		if old != nil {
			if err := db.deleteSessionExpire(tx, old); err != nil {
				return err
			}
		}

		return db.session(tx).delete([]byte(id))
	})
}
