package server

import (
	"fmt"
	"net/http"

	"cc/internal/puzzles"
)

func eventsMiddleware(events []puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		maxLen := 0
		for _, event := range events {
			if len(event.Name) > maxLen {
				maxLen = len(event.Name)
			}
		}
		pd.EventAlign = maxLen + 2

		pd.Events = make([]eventData, 0, len(events))
		for _, event := range events {
			// TODO solved from user

			evd := eventData{
				Path:  event.Path,
				Name:  event.Name,
				Total: event.Total,
			}

			pd.Events = append(pd.Events, evd)
		}

		next.ServeHTTP(w, r)
	})
}

func puzzlesMiddleware(event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Event = eventData{
			Path: event.Path,
			Name: event.Name,
		}

		maxLen := 0
		for _, puzzle := range event.Puzzles {
			if len(puzzle.Name) > maxLen {
				maxLen = len(puzzle.Name)
			}
		}
		pd.PuzzleAlign = maxLen + 2

		pd.Puzzles = make([]puzzleData, 0, len(event.Puzzles))
		for _, puzzle := range event.Puzzles {
			pzd := puzzleData{
				Path: puzzle.Path,
				Name: puzzle.Name,
			}

			// TODO handle user solved classes

			if puzzle.Unlock.Before(pd.Now) {
				pzd.Class = puzzleClassUnlocked
			} else {
				pzd.Class = puzzleClassLocked
				pzd.Unlock = &puzzle.Unlock
			}

			pd.Puzzles = append(pd.Puzzles, pzd)
		}

		next.ServeHTTP(w, r)
	})
}

func puzzleDataFunc(puzzle puzzles.Puzzle, locked dataFunc) dataFunc {
	return func(r *http.Request) (int, any) {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = puzzle.Path
		pd.Title = puzzle.Name

		if puzzle.Unlock.After(pd.Now) {
			return locked(r)
		}

		// TODO handle user solved parts

		pd.Content.Parts = make([]partData, 0, len(puzzle.Parts))
		for _, part := range puzzle.Parts {
			pd.Content.Parts = append(pd.Content.Parts, partData{
				MD:         part.Text,
				WantAnswer: true,
			})

			// TODO if not solved { break }
		}

		return http.StatusOK, pd
	}
}

func latestPuzzleRedirect(event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		for i := len(event.Puzzles) - 1; i >= 0; i-- {
			puzzle := event.Puzzles[i]
			if puzzle.Unlock.After(pd.Now) {
				continue
			}

			http.Redirect(w, r, fmt.Sprintf("/%s/puzzle/%s", event.Path, puzzle.Path), http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%s", event.Path), http.StatusTemporaryRedirect)
	})
}

func inputIndex(userOffset int, puzzle puzzles.Puzzle) int {
	return (userOffset + puzzle.InputOffset) % len(puzzle.Inputs)
}

func puzzleInputHandler(puzzle puzzles.Puzzle, locked http.Handler) http.Handler {
	handlers := make([]http.Handler, len(puzzle.Inputs))
	for i, input := range puzzle.Inputs {
		handlers[i] = cachedHandler([]byte(input.Text), "text/plain; charset=utf-8")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		if puzzle.Unlock.After(pd.Now) {
			locked.ServeHTTP(w, r)
			return
		}

		if pd.User == nil {
			http.NotFound(w, r)
			return
		}

		i := inputIndex(pd.User.InputOffset, puzzle)
		handlers[i].ServeHTTP(w, r)
	})
}

func puzzleAnswerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO if not logged in, return 404

		// TODO handle answer
	})
}
