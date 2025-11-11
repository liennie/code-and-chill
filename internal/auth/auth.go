// Package auth provides authentication helpers and types for the application.
package auth

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"cc/internal/db"
)

type Auth struct {
	Discord DiscordAuth
	db      *db.DB
}

func New(config Config, db *db.DB) *Auth {
	return &Auth{
		Discord: newDiscordAuth(config.Discord, db),
		db:      db,
	}
}

type User struct {
	ID          string
	Name        string
	AvatarURL   string
	InputOffset int
}

func newID() string {
	return strconv.FormatUint(rand.Uint64(), 10)
}

func userFromDB(id string, s *db.User) *User {
	return &User{
		ID:        id,
		Name:      s.Name,
		AvatarURL: s.AvatarURL,
	}
}

func (u *User) toDB() *db.User {
	return &db.User{
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}
}

func (a *Auth) View(id string, f func(user *User) error) error {
	return a.db.View(func(tx *db.Tx) error {
		bucket := tx.User()

		user := bucket.Get(id)
		if user == nil {
			return fmt.Errorf("no user with id %q", id)
		}

		if err := f(userFromDB(id, user)); err != nil {
			return err
		}
		return nil
	})
}
