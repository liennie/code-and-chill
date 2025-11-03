package server

import (
	"fmt"
	"net/http"
	"woc/internal/puzzles"
)

func puzzlesMiddleware(year int, puzzles []puzzles.Puzzle, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromRequest(r)
		pd.Year = year

		pd.Puzzles = make([]puzzleData, 0, len(puzzles))
		for _, puzzle := range puzzles {
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
		pd := pageDataFromRequest(r)
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

func latestPuzzleRedirect(year int, puzzles []puzzles.Puzzle) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromRequest(r)

		for i := len(puzzles) - 1; i >= 0; i-- {
			puzzle := puzzles[i]
			if puzzle.Unlock.After(pd.Now) {
				continue
			}

			http.Redirect(w, r, fmt.Sprintf("/%d/puzzle/%d", year, i+1), http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%d", year), http.StatusTemporaryRedirect)
	})
}

func puzzleInputHandler(i int, puzzle puzzles.Puzzle, locked http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromRequest(r)

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
