package auth

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/liennie/code-and-chill/internal/db"
)

type UserNotFoundError struct {
	ID string
}

func (e *UserNotFoundError) Error() string {
	return fmt.Sprintf("no user with id %q", e.ID)
}

type User struct {
	ID           string `json:"-"`
	Name         string `json:"name"`
	AvatarURL    string `json:"avatar_url"`
	AvatarSource string `json:"avatar_source,omitempty"`
	RandomAvatar bool   `json:"random_avatar,omitempty"`
	Token        string `json:"token"`
	Admin        bool   `json:"admin,omitempty"`
	Hidden       bool   `json:"hidden,omitempty"`
	Tester       bool   `json:"tester,omitempty"`
}

var _ db.KeySetter = (*User)(nil)

func (u *User) SetKey(key string) {
	u.ID = key
}

// AvatarPathPrefix is the URL prefix under which cached user avatars are
// served by the HTTP handler. The full URL for a given user is
// AvatarPathPrefix + userID.
const AvatarPathPrefix = "/avatar/"

// UserAvatar holds the cached image data for a user's avatar together with
// its MIME content type and a short ETag derived from the data.
type UserAvatar struct {
	Data        []byte `json:"data"`
	ContentType string `json:"content_type"`
	ETag        string `json:"etag"`
}

type UserProgress struct {
	Incorrect uint                      `json:"incorrect,omitzero"`
	Timeout   time.Time                 `json:"timeout,omitzero"`
	Puzzles   map[string]PuzzleProgress `json:"puzzles"`
}

// MaxIncorrectKept is the maximum number of recent incorrect answers stored
// for the part a user is currently trying to solve.
const MaxIncorrectKept = 5

type PuzzleProgress struct {
	Parts     []PartProgress `json:"parts"`
	Incorrect []string       `json:"incorrect,omitempty"`
}

type PartProgress struct {
	Time time.Time `json:"time"`
}

func newID() string {
	return strconv.FormatUint(rand.Uint64(), 10)
}

func (a *Auth) ListUsers() (map[string]*User, error) {
	return a.FindUsers(func(u *User) bool { return true })
}

func (a *Auth) FindUsers(cond func(*User) bool) (map[string]*User, error) {
	// TODO range start end

	users := map[string]*User{}
	err := a.db.View(func(tx *db.Tx) error {
		bucket := a.bucketUser.Open(tx)
		for id, user := range bucket.All() {
			if cond(user) {
				users[id] = user
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
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
		return nil, &UserNotFoundError{id}
	}
	return user, nil
}

// UserAvatar returns the cached avatar image for the given user id, or nil if
// none is stored.
func (a *Auth) UserAvatar(id string) (*UserAvatar, error) {
	var avatar *UserAvatar
	err := a.db.View(func(tx *db.Tx) error {
		avatar = a.bucketUserAvatar.Open(tx).Get(id)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return avatar, nil
}

func (a *Auth) UpdateUser(id string, f func(*User) error) error {
	return a.db.Update(func(tx *db.Tx) error {
		bucket := a.bucketUser.Open(tx)

		user := bucket.Get(id)
		if user == nil {
			return &UserNotFoundError{id}
		}

		if err := f(user); err != nil {
			return err
		}

		return bucket.Put(id, user)
	})
}

func (a *Auth) AllProgress(event string) (map[*User]*UserProgress, error) {
	allProgress := map[*User]*UserProgress{}
	err := a.db.View(func(tx *db.Tx) error {
		bucket := a.bucketProgress.Open(tx)
		eventBucket := bucket.Bucket(event)
		if eventBucket == nil {
			return nil
		}

		userBucket := a.bucketUser.Open(tx)
		for userID, progress := range eventBucket.All() {
			user := userBucket.Get(userID)
			if user == nil {
				continue
			}
			allProgress[user] = progress
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return allProgress, nil
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

// DeleteProgress removes the user's progress entry from every event.
func (a *Auth) DeleteProgress(user string) error {
	return a.db.Update(func(tx *db.Tx) error {
		if u := a.bucketUser.Open(tx).Get(user); u == nil {
			return &UserNotFoundError{user}
		}

		bucket := a.bucketProgress.Open(tx)
		for event := range bucket.Keys() {
			eventBucket := bucket.Bucket(event)
			if eventBucket == nil {
				continue
			}
			if err := eventBucket.Delete(user); err != nil {
				return err
			}
		}
		return nil
	})
}
