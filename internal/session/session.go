// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"time"

	"cc/internal/db"
)

type Data struct {
	Auth *Auth `json:"auth,omitempty"`
	User *User `json:"user,omitempty"`
}

type Auth struct {
	State  string `json:"state"`
	Event  string `json:"event"`
	Return string `json:"return,omitempty"`
}

type User struct {
	ID string `json:"id"`
}

type ID struct {
	ID     string    `json:"-"`
	Expire time.Time `json:"expire"`
	Update bool      `json:"-"`
}

var _ db.KeySetter = (*ID)(nil)

func (id *ID) SetKey(key string) {
	id.ID = key
}

type Session struct {
	id   *ID
	data *Data

	db                *db.DB
	bucketSessionData *db.BucketKey[Data]
}

func (s *Session) ID() ID {
	return *s.id
}

func (s *Session) View(f func(data *Data) error) error {
	return f(s.data)
}

func (s *Session) Update(f func(data *Data) error) error {
	return s.db.Update(func(tx *db.Tx) error {
		bucket := s.bucketSessionData.Open(tx)

		data := bucket.Get(s.id.ID)
		if data == nil {
			data = s.data
		}

		if err := f(data); err != nil {
			return err
		}
		return bucket.Put(s.id.ID, data)
	})
}
