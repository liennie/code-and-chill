package session

import (
	"time"
)

type Config struct {
	Bits            int           `yaml:"bits"`
	Expire          time.Duration `yaml:"expire"`
	Truncate        time.Duration `yaml:"truncate"`
	CleanupSchedule string        `yaml:"cleanupSchedule"`
}
