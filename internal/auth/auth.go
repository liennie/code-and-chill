// Package auth provides authentication helpers and types for the application.
package auth

type Auth struct {
	Discord DiscordAuth
}

func New(config Config) *Auth {
	return &Auth{
		Discord: newDiscordAuth(config.Discord),
	}
}

type User struct {
	ID        string
	Username  string
	AvatarURL string
}
