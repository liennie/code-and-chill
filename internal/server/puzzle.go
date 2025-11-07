package server

import (
	"fmt"
	"net/http"
	"woc/internal/puzzles"
)

func eventsMiddleware(events []puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		pd.Events = make([]eventData, 0, len(events))
		for _, event := range events {
			evd := eventData{
				Path: event.Path,
				Name: event.Name,
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

		pd.Puzzles = make([]puzzleData, 0, len(event.Puzzles))
		for _, puzzle := range event.Puzzles {
			pzd := puzzleData{
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

func puzzleDataFunc(i int, puzzle puzzles.Puzzle, locked dataFunc) dataFunc {
	return func(r *http.Request) (int, any) {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = i
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

			http.Redirect(w, r, fmt.Sprintf("/%s/puzzle/%d", event.Path, i+1), http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%s", event.Path), http.StatusTemporaryRedirect)
	})
}

func puzzleInputHandler(i int, puzzle puzzles.Puzzle, locked http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		if puzzle.Unlock.After(pd.Now) {
			locked.ServeHTTP(w, r)
			return
		}

		// TODO if not logged in, return 404

		// TODO return user input
	})
}

func puzzleAnswerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO if not logged in, return 404

		// TODO handle answer
	})
}
