package session

import (
	"time"

	"cc/internal/db"
)

type ID struct {
	ID     string
	Expire time.Time
	Update bool
}

func sessionFromDB(id string, s *db.Session) *ID {
	return &ID{
		ID:     id,
		Expire: s.Expire,
	}
}

func (s *ID) toDB() *db.Session {
	return &db.Session{
		Expire: s.Expire,
	}
}
