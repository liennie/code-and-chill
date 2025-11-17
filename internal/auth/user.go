package auth

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"cc/internal/db"
)

type User struct {
	ID          string `json:"-"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	InputOffset uint8  `json:"input_offset"`
}

var _ db.KeySetter = (*User)(nil)

func (u *User) SetKey(key string) {
	u.ID = key
}

type UserProgress struct {
	Puzzles map[string]PuzzleProgress `json:"puzzles"`
}

type PuzzleProgress struct {
	Parts []PartProgress `json:"parts"`
}

type PartProgress struct {
	Time   time.Time `json:"time"`
	Answer string    `json:"answer"`
}

func newID() string {
	return strconv.FormatUint(rand.Uint64(), 10)
}

func (a *Auth) View(id string, f func(user *User) error) error {
	return a.db.View(func(tx *db.Tx) error {
		bucket := a.bucketUser.Open(tx)

		user := bucket.Get(id)
		if user == nil {
			return fmt.Errorf("no user with id %q", id)
		}

		if err := f(user); err != nil {
			return err
		}
		return nil
	})
}
