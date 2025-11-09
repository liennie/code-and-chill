// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"time"

	"cc/internal/db"

	"github.com/robfig/cron/v3"
)

type Store struct {
	bytes    int
	expire   time.Duration
	truncate time.Duration

	db      *db.DB
	cleanup *cron.Cron
}

func NewStore(config Config, db *db.DB) *Store {
	schedule, err := cron.ParseStandard(config.CleanupSchedule)
	if err != nil {
		panic(fmt.Errorf("cron schedule: %w", err))
	}

	s := &Store{
		bytes:    config.Bits / 8,
		expire:   config.Expire,
		truncate: config.Truncate,

		db: db,
	}

	s.cleanup = cron.New(
		cron.WithLogger(&slogLogger{}),
	)
	s.cleanup.Schedule(schedule, &cleanupJob{s})
	s.cleanup.Start()

	return s
}

func (s *Store) Close() error {
	<-s.cleanup.Stop().Done()
	return nil
}

func (s *Store) Init(id string, now time.Time) (session *SessionID, err error) {
	err = s.db.Update(func(tx *db.Tx) (err error) {
		sessionBucket := tx.Session()

		if id != "" {
			dbs := sessionBucket.Get(id)
			if dbs != nil && dbs.Expire.After(now) {
				session = sessionFromDB(id, dbs)
				err = s.updateExpire(tx, session, now)
				if err != nil {
					return err
				}
				if session.Update {
					return sessionBucket.Put(id, session.toDB())
				}
				return nil
			}
		}

		id = s.newID()
		for sessionBucket.Has(id) {
			id = s.newID()
		}

		session = &SessionID{ID: id}
		err = s.newExpire(tx, session, now)
		if err != nil {
			return err
		}
		return sessionBucket.Put(id, session.toDB())
	})
	return
}

func (s *Store) newID() string {
	buf := make([]byte, s.bytes)
	rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func deleteExpire(expireBucket *db.Bucket[db.SessionExpire], id string, expire string) error {
	sessions := expireBucket.Get(expire)
	if sessions == nil {
		return nil
	}

	sessions.IDs = slices.DeleteFunc(sessions.IDs, func(v string) bool { return v == id })
	if len(sessions.IDs) == 0 {
		return expireBucket.Delete(expire)
	} else {
		return expireBucket.Put(expire, sessions)
	}
}

func addExpire(expireBucket *db.Bucket[db.SessionExpire], id string, expire string) error {
	sessions := expireBucket.Get(expire)
	if sessions == nil {
		sessions = &db.SessionExpire{
			IDs: []string{id},
		}
	} else {
		sessions.IDs = append(sessions.IDs, id)
	}

	return expireBucket.Put(expire, sessions)
}

func expireKey(expire time.Time) string {
	return expire.UTC().Format(time.RFC3339)
}

func (s *Store) newExpire(tx *db.Tx, session *SessionID, now time.Time) error {
	session.Expire = now.Add(s.expire).Truncate(s.truncate)
	expire := expireKey(session.Expire)

	err := addExpire(tx.SessionExpire(), session.ID, expire)
	if err != nil {
		return err
	}

	session.Update = true
	return nil
}

func (s *Store) updateExpire(tx *db.Tx, session *SessionID, now time.Time) error {
	old := expireKey(session.Expire)
	session.Expire = now.Add(s.expire).Truncate(s.truncate)
	new := expireKey(session.Expire)

	if old == new {
		return nil
	}

	expireBucket := tx.SessionExpire()
	err := deleteExpire(expireBucket, session.ID, old)
	if err != nil {
		return err
	}

	err = addExpire(expireBucket, session.ID, new)
	if err != nil {
		return err
	}

	session.Update = true
	return nil
}
