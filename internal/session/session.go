package session

import (
	"time"

	"cc/internal/db"
)

type Session struct {
	ID     string
	Expire time.Time
}

func sessionFromDB(s *db.Session) Session {
	return Session{
		ID:     s.ID,
		Expire: s.Expire,
	}
}

func (s Session) toDB() *db.Session {
	return &db.Session{
		ID:     s.ID,
		Expire: s.Expire,
	}
}

func shortKey(id string) string {
	if len(id) <= 16 {
		return "..."
	}
	return id[:4] + "..." + id[len(id)-4:]
}
