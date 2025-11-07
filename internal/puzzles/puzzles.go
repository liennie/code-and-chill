package puzzles

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
	"woc/internal/ctxlog"

	"github.com/goccy/go-yaml"
)

type Puzzles struct {
	Default Event
	Events  []Event
}

func Load(config Config) *Puzzles {
	p := &Puzzles{
		Events: make([]Event, 0, len(config.Events)),
	}

	for _, event := range config.Events {
		ec, err := loadEventConfig(event.Config)
		if err != nil {
			panic(fmt.Sprintf("puzzles: load event %q: %v", event, err))
		}

		fsys := os.DirFS(filepath.Dir(filepath.FromSlash(event.Config)))

		e := Event{
			Path:    event.Path,
			Name:    ec.Name,
			Puzzles: make([]Puzzle, 0, len(ec.Puzzles)),
		}
		for _, puzzle := range ec.Puzzles {
			pz := Puzzle{
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
					Text:    string(content),
					Answers: input.Answers,
				})
			}

			e.Puzzles = append(e.Puzzles, pz)
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
	Path    string
	Name    string
	Puzzles []Puzzle
}

type Puzzle struct {
	Name   string
	Unlock time.Time
	Parts  []Part
	Inputs []Input
}

type Part struct {
	Text string
	ID   string
}

type Input struct {
	Text    string
	Answers []string
}
