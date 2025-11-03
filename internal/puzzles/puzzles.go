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
	DefaultYear int
	Years       map[int]Year
}

func Load(config Config) *Puzzles {
	p := &Puzzles{
		DefaultYear: config.DefaultYear,
		Years:       make(map[int]Year, len(config.Years)),
	}

	for year, path := range config.Years {
		yc, err := loadYearConfig(path)
		if err != nil {
			panic(fmt.Sprintf("puzzles: load year %d: %v", year, err))
		}

		fsys := os.DirFS(filepath.Dir(filepath.FromSlash(path)))

		y := Year{
			Puzzles: make([]Puzzle, 0, len(yc.Puzzles)),
		}
		for _, puzzle := range yc.Puzzles {
			pz := Puzzle{
				Name:   puzzle.Name,
				Unlock: puzzle.Unlock,
				Parts:  make([]Part, 0, len(puzzle.Parts)),
				Inputs: make([]Input, 0, len(puzzle.Inputs)),
			}

			for i, part := range puzzle.Parts {
				content, err := fs.ReadFile(fsys, part.File)
				if err != nil {
					panic(fmt.Errorf("puzzles: year %d part %d read data file %q: %w", year, i, part.File, err))
				}

				pz.Parts = append(pz.Parts, Part{
					Text: string(content),
					ID:   part.ID,
				})
			}

			for i, input := range puzzle.Inputs {
				if len(input.Answers) != len(puzzle.Parts) {
					panic(fmt.Errorf(
						"puzzles: year %d puzzle %q: number of input %d answers (%d) does not match number of parts (%d)",
						year,
						puzzle.Name,
						i,
						len(input.Answers),
						len(puzzle.Parts),
					))
				}

				content, err := fs.ReadFile(fsys, input.File)
				if err != nil {
					panic(fmt.Errorf("puzzles: year %d input %d read data file %q: %w", year, i, input.File, err))
				}

				pz.Inputs = append(pz.Inputs, Input{
					Text:    string(content),
					Answers: input.Answers,
				})
			}

			y.Puzzles = append(y.Puzzles, pz)
		}
		p.Years[year] = y
	}

	return p
}

func loadYearConfig(filename string) (YearConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return YearConfig{}, fmt.Errorf("open %q: %w", filename, err)
	}
	defer ctxlog.Close(context.Background(), filename, file)

	dec := yaml.NewDecoder(file, yaml.Strict())

	var config YearConfig
	err = dec.Decode(&config)
	if err != nil {
		return YearConfig{}, fmt.Errorf("yaml: %w", err)
	}

	return config, nil
}

type Year struct {
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
