package session

import (
	"time"

	"cc/internal/db"
)

type SessionID struct {
	ID     string
	Expire time.Time
	Update bool
}

func sessionFromDB(id string, s *db.Session) *SessionID {
	return &SessionID{
		ID:     id,
		Expire: s.Expire,
	}
}

func (s *SessionID) toDB() *db.Session {
	return &db.Session{
		Expire: s.Expire,
	}
}
