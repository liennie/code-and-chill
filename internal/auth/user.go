package auth

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"cc/internal/db"
)

type User struct {
	ID        string `json:"-"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

var _ db.KeySetter = (*User)(nil)

func (u *User) SetKey(key string) {
	u.ID = key
}

type UserProgress struct {
	Incorrect uint                      `json:"incorrect,omitzero"`
	Timeout   time.Time                 `json:"timeout,omitzero"`
	Puzzles   map[string]PuzzleProgress `json:"puzzles"`
}

type PuzzleProgress struct {
	Parts []PartProgress `json:"parts"`
}

type PartProgress struct {
	Time time.Time `json:"time"`
}

func newID() string {
	return strconv.FormatUint(rand.Uint64(), 10)
}

func (a *Auth) User(id string) (*User, error) {
	var user *User
	err := a.db.View(func(tx *db.Tx) error {
		user = a.bucketUser.Open(tx).Get(id)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("no user with id %q", id)
	}
	return user, nil
}

func (a *Auth) Progress(event, user string) (*UserProgress, error) {
	var progress *UserProgress
	err := a.db.View(func(tx *db.Tx) error {
		bucket := a.bucketProgress.Open(tx)
		eventBucket := bucket.Bucket(event)
		if eventBucket == nil {
			return nil
		}

		progress = eventBucket.Get(user)
		return nil
	})
	return progress, err
}

func (a *Auth) UpdateProgress(event, user string, f func(*UserProgress) error) error {
	return a.db.Update(func(tx *db.Tx) error {
		bucket := a.bucketProgress.Open(tx)
		eventBucket, err := bucket.CreateBucket(event)
		if err != nil {
			return err
		}

		progress := eventBucket.Get(user)
		if progress == nil {
			progress = &UserProgress{
				Puzzles: map[string]PuzzleProgress{},
			}
		}

		if err := f(progress); err != nil {
			return err
		}

		return eventBucket.Put(user, progress)
	})
}
