package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"time"

	"cc/internal/cronlog"
	"cc/internal/db"

	"github.com/robfig/cron/v3"
)

type Store struct {
	bytes    int
	expire   time.Duration
	truncate time.Duration

	db                  *db.DB
	bucketSession       *db.BucketKey[Session]
	bucketSessionExpire *db.BucketKey[dbSessionExpire]

	cleanup *cron.Cron
}

func NewStore(config Config, ddb *db.DB) *Store {
	if config.Bits == 0 {
		config.Bits = 256
	}
	if config.Expire == 0 {
		config.Expire = 30 * 24 * time.Hour
	}
	if config.Truncate == 0 {
		config.Truncate = 24 * time.Hour
	}
	if config.CleanupSchedule == "" {
		panic("session: cleanupSchedule is required")
	}

	schedule, err := cron.ParseStandard(config.CleanupSchedule)
	if err != nil {
		panic(fmt.Errorf("session: cleanupSchedule: %w", err))
	}

	s := &Store{
		bytes:    config.Bits / 8,
		expire:   config.Expire,
		truncate: config.Truncate,

		db:                  ddb,
		bucketSession:       db.NewBucketKey[Session](ddb, db.BucketSession),
		bucketSessionExpire: db.NewBucketKey[dbSessionExpire](ddb, db.BucketSessionExpire),
	}

	s.cleanup = cron.New(
		cron.WithLogger(cronlog.NewSlogLogger()),
	)
	s.cleanup.Schedule(schedule, &cleanupJob{s})
	s.cleanup.Start()

	return s
}

func (s *Store) Close() error {
	<-s.cleanup.Stop().Done()
	return nil
}

func (s *Store) Init(id string, now time.Time) (session *Session, err error) {
	err = s.db.Update(func(tx *db.Tx) (err error) {
		sessionBucket := s.bucketSession.Open(tx)

		if id != "" {
			session = sessionBucket.Get(id)
			if session != nil && session.id.Expire.After(now) {
				session.db = s.db
				session.bucketSession = s.bucketSession

				err = s.updateExpire(tx, &session.id, now)
				if err != nil {
					return err
				}

				if session.id.Update {
					return sessionBucket.Put(id, session)
				}
				return nil
			}
		}

		id = s.newID()
		for sessionBucket.Has(id) {
			id = s.newID()
		}

		session = &Session{
			id: ID{ID: id},

			db:            s.db,
			bucketSession: s.bucketSession,
		}

		err = s.newExpire(tx, &session.id, now)
		if err != nil {
			return err
		}

		return sessionBucket.Put(id, session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Store) newID() string {
	buf := make([]byte, s.bytes)
	rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

type dbSessionExpire struct {
	IDs []string `json:"ids"`
}

func deleteExpire(expireBucket *db.Bucket[dbSessionExpire], id string, expire string) error {
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

func addExpire(expireBucket *db.Bucket[dbSessionExpire], id string, expire string) error {
	sessions := expireBucket.Get(expire)
	if sessions == nil {
		sessions = &dbSessionExpire{
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

func (s *Store) newExpire(tx *db.Tx, session *ID, now time.Time) error {
	session.Expire = now.Add(s.expire).Truncate(s.truncate)
	expire := expireKey(session.Expire)

	err := addExpire(s.bucketSessionExpire.Open(tx), session.ID, expire)
	if err != nil {
		return err
	}

	session.Update = true
	return nil
}

func (s *Store) updateExpire(tx *db.Tx, session *ID, now time.Time) error {
	old := expireKey(session.Expire)
	session.Expire = now.Add(s.expire).Truncate(s.truncate)
	new := expireKey(session.Expire)

	if old == new {
		return nil
	}

	expireBucket := s.bucketSessionExpire.Open(tx)
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
