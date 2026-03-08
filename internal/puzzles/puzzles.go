// Package puzzles provides types and loading logic for puzzle events,
// their puzzles, parts, and inputs.
package puzzles

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/liennie/code-and-chill/internal/ctxlog"

	"github.com/goccy/go-yaml"
)

type Puzzles struct {
	Name    string
	Default Event
	Events  []Event
}

func Load(config Config) *Puzzles {
	if config.Name == "" {
		panic("puzzles: name is required")
	}
	if len(config.Events) == 0 {
		panic("puzzles: at least one event is required")
	}
	if config.Default == "" {
		config.Default = config.Events[len(config.Events)-1].Path
	}

	p := &Puzzles{
		Name:   config.Name,
		Events: make([]Event, 0, len(config.Events)),
	}

	eventPaths := map[string]struct{}{}
	eventIDs := map[string]struct{}{}

	for _, event := range config.Events {
		if _, exists := eventPaths[event.Path]; exists {
			panic(fmt.Errorf("puzzles: duplicate event path %q", event.Path))
		}
		eventPaths[event.Path] = struct{}{}

		e := LoadEvent(event)
		if _, exists := eventIDs[e.ID]; exists {
			panic(fmt.Errorf("puzzles: duplicate event ID %q", e.ID))
		}
		eventIDs[e.ID] = struct{}{}

		p.Events = append(p.Events, e)
		if config.Default == e.Path {
			p.Default = e
		}
	}

	if p.Default.Path == "" {
		panic(fmt.Errorf("puzzles: default event %q not found", config.Default))
	}

	return p
}

func LoadEvent(event EventPathConfig) Event {
	ec, err := loadEventConfig(event.Config)
	if err != nil {
		panic(fmt.Errorf("puzzles: load event %q: %w", event.Config, err))
	}
	if event.Path == "" {
		panic(fmt.Errorf("puzzles: event %q has empty path", event.Config))
	}
	if ec.ID == "" {
		panic(fmt.Errorf("puzzles: event %q has empty ID", event.Config))
	}

	fsys := os.DirFS(filepath.Dir(filepath.FromSlash(event.Config)))

	e := Event{
		ID:      ec.ID,
		Path:    event.Path,
		Name:    ec.Name,
		Puzzles: make([]Puzzle, 0, len(ec.Puzzles)),
		Total:   0,
	}

	puzzlePaths := map[string]struct{}{}
	puzzleIDs := map[string]struct{}{}

	for _, puzzle := range ec.Puzzles {
		pc, err := loadPuzzleConfig(fsys, puzzle.Config)
		if err != nil {
			panic(fmt.Errorf("puzzles: event %q load puzzle %q: %w", event.Config, puzzle.Config, err))
		}
		if pc.Path == "" {
			panic(fmt.Errorf("puzzles: event %q puzzle %q has empty path", event.Config, puzzle.Config))
		}
		if _, exists := puzzlePaths[pc.Path]; exists {
			panic(fmt.Errorf("puzzles: event %q duplicate puzzle path %q", event.Config, pc.Path))
		}
		puzzlePaths[pc.Path] = struct{}{}
		if pc.ID == "" {
			panic(fmt.Errorf("puzzles: event %q puzzle %q has empty ID", event.Config, puzzle.Config))
		}
		if _, exists := puzzleIDs[pc.ID]; exists {
			panic(fmt.Errorf("puzzles: event %q duplicate puzzle ID %q", event.Config, pc.ID))
		}
		puzzleIDs[pc.ID] = struct{}{}

		fsys, err := fs.Sub(fsys, path.Dir(puzzle.Config))
		if err != nil {
			panic(fmt.Errorf("puzzles: event %q puzzle %q sub fs: %w", event.Config, puzzle.Config, err))
		}

		pz := Puzzle{
			ID:     pc.ID,
			Path:   pc.Path,
			Name:   pc.Name,
			Unlock: puzzle.Unlock,
			Parts:  make([]Part, 0, len(pc.Parts)),
			Inputs: make([]Input, 0, len(pc.Inputs)),
		}

		for i, part := range pc.Parts {
			content, err := fs.ReadFile(fsys, part.File)
			if err != nil {
				panic(fmt.Errorf("puzzles: event %q puzzle %q part %d read data file %q: %w", event.Config, puzzle.Config, i, part.File, err))
			}

			pz.Parts = append(pz.Parts, Part{
				Text: string(content),
				ID:   part.ID,
			})
		}

		for i, input := range pc.Inputs {
			if len(input.Answers) != len(pc.Parts) {
				panic(fmt.Errorf(
					"puzzles: event %q puzzle %q number of input %d answers (%d) does not match number of parts (%d)",
					event.Config,
					puzzle.Config,
					i,
					len(input.Answers),
					len(pc.Parts),
				))
			}

			content, err := fs.ReadFile(fsys, input.File)
			if err != nil {
				panic(fmt.Errorf("puzzles: event %q puzzle %q input %d read data file %q: %w", event.Config, puzzle.Config, i, input.File, err))
			}

			pz.Inputs = append(pz.Inputs, Input{
				File:    input.File,
				Text:    content,
				Answers: input.Answers,
			})
		}

		e.Puzzles = append(e.Puzzles, pz)
		e.Total += len(pz.Parts)
	}

	return e
}

func loadEventConfig(filename string) (EventConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return EventConfig{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.CloseErr(context.Background(), filename, file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config EventConfig
	err = dec.Decode(&config)
	if err != nil {
		return EventConfig{}, fmt.Errorf("yaml: %w", err)
	}

	return config, nil
}

func loadPuzzleConfig(fsys fs.FS, filename string) (PuzzleConfig, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return PuzzleConfig{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.CloseErr(context.Background(), filename, file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config PuzzleConfig
	err = dec.Decode(&config)
	if err != nil {
		return PuzzleConfig{}, fmt.Errorf("yaml: %w", err)
	}

	return config, nil
}

type Event struct {
	ID      string
	Path    string
	Name    string
	Puzzles []Puzzle
	Total   int
}

type Puzzle struct {
	ID     string
	Path   string
	Name   string
	Unlock time.Time
	Parts  []Part
	Inputs []Input
}

type Part struct {
	ID   string
	Text string
}

type Input struct {
	File    string
	Text    []byte
	Answers []string
}
