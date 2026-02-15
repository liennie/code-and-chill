package notifier

type Config struct {
	BaseURI string         `yaml:"baseURI"`
	Discord *DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
	Token    string                   `yaml:"token"`
	Channels map[string]ChannelConfig `yaml:"channels"`
}

type ChannelConfig struct {
	Notifications string `yaml:"notifications"`
	Forum         string `yaml:"forum"`
}
