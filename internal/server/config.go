package server

import (
	"time"
)

type Config struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	TLSCertFile       string        `yaml:"tlsCertFile"`
	TLSKeyFile        string        `yaml:"tlsKeyFile"`
	TLSReloadSchedule string        `yaml:"tlsReloadSchedule"`
	HTTPSRedirect     bool          `yaml:"httpsRedirect"`
	DataDir           string        `yaml:"dataDir"`
	ShutdownTimeout   time.Duration `yaml:"shutdownTimeout"`
}
