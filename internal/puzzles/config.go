package puzzles

import (
	"time"
)

type Config struct {
	DefaultYear int            `yaml:"defaultYear"`
	Years       map[int]string `yaml:"years"`
}

type YearConfig struct {
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
