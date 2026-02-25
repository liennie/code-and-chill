package auth

import (
	"time"
)

type Config struct {
	Discord DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
	ClientID        string        `yaml:"clientID"`
	ClientSecret    string        `yaml:"clientSecret"`
	RedirectURI     string        `yaml:"redirectURI"`
	ExchangeTimeout time.Duration `yaml:"exchangeTimeout"`
	GuildID         string        `yaml:"guildID"`
}
