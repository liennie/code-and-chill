// Package puzzles provides types and loading logic for puzzle events,
// their puzzles, parts, and inputs.
package puzzles

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"cc/internal/ctxlog"

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
		ec, err := loadEventConfig(event.Config)
		if err != nil {
			panic(fmt.Sprintf("puzzles: load event %q: %v", event.Config, err))
		}
		if event.Path == "" {
			panic(fmt.Sprintf("puzzles: event with config %q has empty path", event.Config))
		}
		if _, exists := eventPaths[event.Path]; exists {
			panic(fmt.Sprintf("puzzles: duplicate event path %q", event.Path))
		}
		eventPaths[event.Path] = struct{}{}
		if ec.ID == "" {
			panic(fmt.Sprintf("puzzles: event with config %q has empty ID", event.Config))
		}
		if _, exists := eventIDs[ec.ID]; exists {
			panic(fmt.Sprintf("puzzles: duplicate event ID %q", ec.ID))
		}
		eventIDs[ec.ID] = struct{}{}

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

		for i, puzzle := range ec.Puzzles {
			if puzzle.Path == "" {
				panic(fmt.Sprintf("puzzles: event %q puzzle %d has empty path", ec.Name, i))
			}
			if _, exists := puzzlePaths[puzzle.Path]; exists {
				panic(fmt.Sprintf("puzzles: event %q: duplicate puzzle path %q", ec.Name, puzzle.Path))
			}
			puzzlePaths[puzzle.Path] = struct{}{}
			if puzzle.ID == "" {
				panic(fmt.Sprintf("puzzles: event %q puzzle with path %q has empty ID", ec.Name, puzzle.Path))
			}
			if _, exists := puzzleIDs[puzzle.ID]; exists {
				panic(fmt.Sprintf("puzzles: event %q: duplicate puzzle ID %q", ec.Name, puzzle.ID))
			}
			puzzleIDs[puzzle.ID] = struct{}{}

			pz := Puzzle{
				ID:     puzzle.ID,
				Path:   puzzle.Path,
				Name:   puzzle.Name,
				Unlock: puzzle.Unlock,
				Parts:  make([]Part, 0, len(puzzle.Parts)),
				Inputs: make([]Input, 0, len(puzzle.Inputs)),
			}

			for i, part := range puzzle.Parts {
				content, err := fs.ReadFile(fsys, part.File)
				if err != nil {
					panic(fmt.Errorf("puzzles: event %q part %d read data file %q: %w", event, i, part.File, err))
				}

				pz.Parts = append(pz.Parts, Part{
					Text: string(content),
					ID:   part.ID,
				})
			}

			for i, input := range puzzle.Inputs {
				if len(input.Answers) != len(puzzle.Parts) {
					panic(fmt.Errorf(
						"puzzles: event %q puzzle %q: number of input %d answers (%d) does not match number of parts (%d)",
						event,
						puzzle.Name,
						i,
						len(input.Answers),
						len(puzzle.Parts),
					))
				}

				content, err := fs.ReadFile(fsys, input.File)
				if err != nil {
					panic(fmt.Errorf("puzzles: event %q input %d read data file %q: %w", event, i, input.File, err))
				}

				pz.Inputs = append(pz.Inputs, Input{
					Text:    content,
					Answers: input.Answers,
				})
			}

			e.Puzzles = append(e.Puzzles, pz)
			e.Total += len(pz.Parts)
		}

		p.Events = append(p.Events, e)
		if config.Default == e.Path {
			p.Default = e
		}
	}

	if p.Default.Path == "" {
		panic(fmt.Sprintf("puzzles: default event %q not found", config.Default))
	}

	return p
}

func loadEventConfig(filename string) (EventConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return EventConfig{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.Close(context.Background(), filename, file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config EventConfig
	err = dec.Decode(&config)
	if err != nil {
		return EventConfig{}, fmt.Errorf("yaml: %w", err)
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
	Text    []byte
	Answers []string
}
