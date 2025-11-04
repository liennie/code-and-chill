// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"sync"
	"time"

	"cc/internal/db"

	"github.com/hashicorp/golang-lru/v2/simplelru"
)

type Store struct {
	bytes  int
	Expire time.Duration

	db    *db.DB
	mx    sync.Mutex
	cache *simplelru.LRU[string, Session]
}

func NewStore(config Config, db *db.DB) *Store {
	s := &Store{
		db:     db,
		Expire: config.Expire,
		bytes:  config.Bits / 8,
	}

	var err error
	s.cache, err = simplelru.NewLRU(config.LRU, func(key string, value Session) {
		err := db.PutSession(value.toDB())
		if err != nil {
			l := slog.Default()
			l.Error("failed to evict session", "id", shortKey(key), "error", err)
		}
	})
	if err != nil {
		panic(errors.New("session: failed to create LRU cache"))
	}

	// TODO expiration goroutine

	return s
}

func (s *Store) Close() error {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.cache.Purge()

	return nil
}

func (s *Store) newID() string {
	buf := make([]byte, s.bytes)
	rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func (s *Store) has(ID string) bool {
	ok := s.cache.Contains(ID)
	if ok {
		return true
	}
	return s.db.HasSession(ID)
}

func (s *Store) Get(ID string, now time.Time) Session {
	s.mx.Lock()
	defer s.mx.Unlock()

	if ID != "" {
		session, ok := s.cache.Get(ID)
		if ok {
			session.Expire = now.Add(s.Expire)
			return session
		}

		dbs := s.db.GetSession(ID)
		if dbs != nil {
			session := sessionFromDB(dbs)
			s.cache.Add(session.ID, session)
			return session
		}
	}

	id := s.newID()
	for s.has(id) {
		id = s.newID()
	}

	session := Session{
		ID:     id,
		Expire: now.Add(s.Expire),
	}
	s.cache.Add(session.ID, session)
	return session
}

func (s *Store) Update(sess Session) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.cache.Add(sess.ID, sess)
}
