// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"encoding/json"
	"fmt"
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
	ID    string `json:"id"`
	Token string `json:"token"`
}

type ID struct {
	ID     string
	Expire time.Time
	Update bool
}

type Session struct {
	id   ID
	data Data

	db            *db.DB
	bucketSession *db.BucketKey[Session]
}

type dbSession struct {
	Expire time.Time `json:"expire"`
	Data   Data      `json:"data"`
}

var _ db.KeySetter = (*Session)(nil)
var _ json.Marshaler = (*Session)(nil)
var _ json.Unmarshaler = (*Session)(nil)

func (s *Session) SetKey(key string) {
	s.id.ID = key
}

func (s *Session) MarshalJSON() ([]byte, error) {
	return json.Marshal(&dbSession{
		Expire: s.id.Expire,
		Data:   s.data,
	})
}

func (s *Session) UnmarshalJSON(data []byte) error {
	var ds dbSession
	if err := json.Unmarshal(data, &ds); err != nil {
		return err
	}

	s.id.Expire = ds.Expire
	s.data = ds.Data
	return nil
}

func (s *Session) ID() ID {
	return s.id
}

func (s *Session) Data() Data {
	return s.data
}

func (s *Session) Update(f func(data *Data) error) error {
	return s.db.Update(func(tx *db.Tx) error {
		bucket := s.bucketSession.Open(tx)

		sess := bucket.Get(s.id.ID)
		if sess == nil {
			return fmt.Errorf("no session with id %q", s.id.ID)
		}

		if err := f(&sess.data); err != nil {
			return err
		}
		s.data = sess.data
		return bucket.Put(s.id.ID, sess)
	})
}
