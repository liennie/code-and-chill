// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"crypto/rand"
	"encoding/base64"
	"slices"
	"time"

	"cc/internal/db"
)

type Store struct {
	expire time.Duration
	bytes  int

	db *db.DB
}

func NewStore(config Config, db *db.DB) *Store {
	s := &Store{
		expire: config.Expire,
		bytes:  config.Bits / 8,

		db: db,
	}

	// TODO expiration goroutine

	return s
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) Get(id string, now time.Time) (session *Session, update bool) {
	err := s.db.Update(func(tx *db.Tx) (err error) {
		bucket := tx.Session()

		if id != "" {
			dbs := bucket.Get(id)
			if dbs != nil {
				session = sessionFromDB(dbs)
				update, err = s.updateExpire(tx, session, now)
				if err != nil {
					return err
				}
				return nil
			}
		}

		id = s.newID()
		for bucket.Has(id) {
			id = s.newID()
		}

		session = &Session{ID: id}
		err = s.newExpire(tx, session, now)
		if err != nil {
			return err
		}
		update = true
		return bucket.Put(id, session.toDB())
	})
	if err != nil {
		panic(err)
	}
	return
}

func (s *Store) newID() string {
	buf := make([]byte, s.bytes)
	rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func deleteExpire(bucket *db.Bucket[db.SessionExpire], id string, expire string) error {
	sessions := bucket.Get(expire)
	if sessions == nil {
		return nil
	}

	sessions.IDs = slices.DeleteFunc(sessions.IDs, func(v string) bool { return v == id })
	if len(sessions.IDs) == 0 {
		return bucket.Delete(expire)
	} else {
		return bucket.Put(expire, sessions)
	}
}

func addExpire(bucket *db.Bucket[db.SessionExpire], id string, expire string) error {
	sessions := bucket.Get(expire)
	if sessions == nil {
		sessions = &db.SessionExpire{
			IDs: []string{id},
		}
	} else {
		sessions.IDs = append(sessions.IDs, id)
	}

	return bucket.Put(expire, sessions)
}

func expireKey(expire time.Time) string {
	return expire.UTC().Format(time.RFC3339)
}

func (s *Store) newExpire(tx *db.Tx, session *Session, now time.Time) error {
	session.Expire = now.Add(s.expire).Truncate(24 * time.Hour)
	expire := expireKey(session.Expire)

	return addExpire(tx.SessionExpire(), session.ID, expire)
}

func (s *Store) updateExpire(tx *db.Tx, session *Session, now time.Time) (bool, error) {
	old := expireKey(session.Expire)
	session.Expire = now.Add(s.expire).Truncate(24 * time.Hour)
	new := expireKey(session.Expire)

	if old == new {
		return false, nil
	}

	bucket := tx.SessionExpire()
	err := deleteExpire(bucket, session.ID, old)
	if err != nil {
		return false, err
	}

	err = addExpire(bucket, session.ID, new)
	if err != nil {
		return false, err
	}

	return true, nil
}
