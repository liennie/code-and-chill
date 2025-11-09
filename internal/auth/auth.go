// Package auth provides authentication helpers and types for the application.
package auth

import (
	"math/rand/v2"
	"strconv"

	"cc/internal/db"
)

type Auth struct {
	Discord DiscordAuth
}

func New(config Config, db *db.DB) *Auth {
	return &Auth{
		Discord: newDiscordAuth(config.Discord, db),
	}
}

type User struct {
	ID        string
	Username  string
	AvatarURL string
}

func newID() string {
	return strconv.FormatUint(rand.Uint64(), 10)
}
