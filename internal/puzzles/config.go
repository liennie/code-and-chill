package puzzles

import (
	"time"
)

type Config struct {
	Default string            `yaml:"default"`
	Events  []EventPathConfig `yaml:"events"`
}

type EventPathConfig struct {
	Path   string `yaml:"path"`
	Config string `yaml:"config"`
}

type EventConfig struct {
	Name    string         `yaml:"name"`
	Puzzles []PuzzleConfig `yaml:"puzzles"`
}

type PuzzleConfig struct {
	Name   string        `yaml:"name"`
	Unlock time.Time     `yaml:"unlock"`
	Parts  []PartConfig  `yaml:"parts"`
	Inputs []InputConfig `yaml:"inputs"`
}

type PartConfig struct {
	File string `yaml:"file"`
	ID   string `yaml:"id"`
}

type InputConfig struct {
	File    string   `yaml:"file"`
	Answers []string `yaml:"answers"`
}
