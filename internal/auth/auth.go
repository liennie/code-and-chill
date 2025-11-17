// Package auth provides authentication helpers and user types for the application.
package auth

import (
	"cc/internal/db"
)

type Auth struct {
	Discord DiscordAuth

	db         *db.DB
	bucketUser *db.BucketKey[User]
}

func New(config Config, ddb *db.DB) *Auth {
	return &Auth{
		Discord: newDiscordAuth(config.Discord, ddb),

		db:         ddb,
		bucketUser: db.NewBucketKey[User](ddb, db.BucketUser),
	}
}
