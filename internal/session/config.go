package session

import (
	"time"
)

type Config struct {
	Bits   int           `yaml:"bits"`
	LRU    int           `yaml:"lru"`
	Expire time.Duration `yaml:"expire"`
}
