// Package eventtest provides helpers for testing puzzle events.
package eventtest

import (
	"testing"

	"github.com/liennie/code-and-chill/internal/puzzles"
)

func Test(t *testing.T, eventConfig string, solutions ...func(input []byte) []string) {
	event := puzzles.LoadEvent(puzzles.EventPathConfig{
		Path:   "test",
		Config: eventConfig,
	})

	if len(solutions) != len(event.Puzzles) {
		t.Fatalf("expected %d solutions, got %d", len(event.Puzzles), len(solutions))
	}

	for i, puzzle := range event.Puzzles {
		for _, input := range puzzle.Inputs {
			answers := solutions[i](input.Text)
			if len(answers) != len(input.Answers) {
				t.Errorf("puzzle %q input %q: expected %d answers, got %d", puzzle.Name, input.File, len(input.Answers), len(answers))
				continue
			}

			for j, answer := range input.Answers {
				if answers[j] != answer {
					t.Errorf("puzzle %q input %q answer %d: expected %q, got %q", puzzle.Name, input.File, j, answer, answers[j])
				}
			}
		}
	}
}
