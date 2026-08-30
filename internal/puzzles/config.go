package puzzles

import (
	"time"
)

type Config struct {
	Name    string            `yaml:"name"`
	Default string            `yaml:"default"`
	Events  []EventPathConfig `yaml:"events"`
}

type EventPathConfig struct {
	Path   string `yaml:"path"`
	Config string `yaml:"config"`
}

type EventConfig struct {
	ID       string             `yaml:"id"`
	Name     string             `yaml:"name"`
	Puzzles  []PuzzlePathConfig `yaml:"puzzles"`
	Contacts []ContactConfig    `yaml:"contacts"`
}

type ContactConfig struct {
	Title   string `yaml:"title"`
	Link    string `yaml:"link"`
	Private bool   `yaml:"private"`
}

type PuzzlePathConfig struct {
	Unlock time.Time `yaml:"unlock"`
	Config string    `yaml:"config"`
}

type PuzzleConfig struct {
	ID     string        `yaml:"id"`
	Path   string        `yaml:"path"`
	Name   string        `yaml:"name"`
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
