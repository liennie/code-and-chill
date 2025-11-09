// Package session provides session management, storage and helpers for creating,
// retrieving and persisting session objects used by the application.
package session

import (
	"cc/internal/db"
	"fmt"
)

type Session struct {
	id *ID
	db *db.DB
}

func (s *Session) ID() ID {
	return *s.id
}

func (s *Session) View(f func(data *Data) error) error {
	return s.db.View(func(tx *db.Tx) error {
		bucket := db.SessionData[Data](tx)

		data := bucket.Get(s.id.ID)
		if data == nil {
			return fmt.Errorf("no data for this session")
		}

		if err := f(data); err != nil {
			return err
		}
		return nil
	})
}

func (s *Session) Update(f func(data *Data) error) error {
	return s.db.Update(func(tx *db.Tx) error {
		bucket := db.SessionData[Data](tx)

		data := bucket.Get(s.id.ID)
		if data == nil {
			return fmt.Errorf("no data for this session")
		}

		if err := f(data); err != nil {
			return err
		}
		return bucket.Put(s.id.ID, data)
	})
}
