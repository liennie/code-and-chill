package main

import (
	"context"
	"fmt"
	"os"
	"woc/internal/ctxlog"
	"woc/internal/db"
	"woc/internal/server"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server server.Config
	DB     db.Config
}

func LoadConfig(ctx context.Context, filename string) (Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.Close(ctx, "config file", file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config Config
	err = dec.Decode(&config)
	if err != nil {
		return Config{}, fmt.Errorf("yaml: %w", err)
	}

	return config, nil
}
