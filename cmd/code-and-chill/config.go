package main

import (
	"context"
	"fmt"
	"os"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/db"
	"github.com/liennie/code-and-chill/internal/notifier"
	"github.com/liennie/code-and-chill/internal/puzzles"
	"github.com/liennie/code-and-chill/internal/server"
	"github.com/liennie/code-and-chill/internal/session"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server   server.Config   `yaml:"server"`
	DB       db.Config       `yaml:"db"`
	Session  session.Config  `yaml:"session"`
	Auth     auth.Config     `yaml:"auth"`
	Notifier notifier.Config `yaml:"notifier"`
	Puzzles  puzzles.Config  `yaml:"puzzles"`
}

func LoadConfig(ctx context.Context, filename string) (Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.CloseErr(ctx, "config file", file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config Config
	err = dec.Decode(&config)
	if err != nil {
		return Config{}, fmt.Errorf("yaml: %w", err)
	}

	return config, nil
}
