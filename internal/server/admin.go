package server

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

func adminMux(admin http.Handler, notadmin http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())

		if user != nil && user.Admin {
			admin.ServeHTTP(w, r)
		} else {
			notadmin.ServeHTTP(w, r)
		}
	})
}

func adminPuzzleProgressData(puzzle puzzles.Puzzle, userID string, progress *auth.UserProgress) puzzleProgressData {
	ii := inputIndex(userID, puzzle)

	ppd := puzzleProgressData{
		Path:       puzzle.Path,
		Name:       puzzle.Name,
		Unlock:     puzzle.Unlock,
		Input:      puzzle.Inputs[ii].File,
		InputIndex: ii,
	}
	if progress != nil {
		for _, t := range progress.Puzzles[puzzle.ID].Parts {
			ppd.Solves = append(ppd.Solves, t.Time)
		}
	}

	return ppd
}

func adminProgressData(a *auth.Auth, event puzzles.Event, userID string) *progressData {
	progress, err := a.Progress(event.ID, userID)
	if err != nil {
		panic(fmt.Errorf("user %q progress: %w", userID, err))
	}
	pd := &progressData{}
	if progress != nil {
		pd.Incorrect = progress.Incorrect
		pd.Timeout = progress.Timeout
	}

	for _, puzzle := range event.Puzzles {
		pd.Puzzles = append(pd.Puzzles, adminPuzzleProgressData(puzzle, userID, progress))
	}

	return pd
}

func adminDataUser(a *auth.Auth, event puzzles.Event, id string) (*adminData, string, bool) {
	ad := &adminData{}

	user, err := a.User(id)
	if err != nil {
		if _, ok := errors.AsType[*auth.ErrUserNotFound](err); ok {
			return nil, "", false
		}
		panic(fmt.Errorf("user %q: %w", id, err))
	}

	ad.User = user
	ad.Progress = adminProgressData(a, event, id)

	return ad, user.Name, true
}

func adminDataPuzzle(a *auth.Auth, event puzzles.Event, puzzle string) (*adminData, string, bool) {
	for _, p := range event.Puzzles {
		if p.Path == puzzle {
			ad := &adminData{
				Puzzle:           &p,
				PuzzleInputUsers: make([][]puzzleInputData, len(p.Inputs)),
			}

			users, err := a.ListUsers()
			if err != nil {
				panic(fmt.Errorf("list users: %w", err))
			}

			keys := slices.SortedFunc(maps.Keys(users), func(a, b string) int {
				return cmp.Compare(
					strings.ToLower(users[a].Name),
					strings.ToLower(users[b].Name),
				)
			})
			for _, id := range keys {
				progress, err := a.Progress(event.ID, id)
				if err != nil {
					panic(fmt.Errorf("user %q progress: %w", id, err))
				}

				ii := inputIndex(id, p)
				ppd := adminPuzzleProgressData(p, id, progress)

				ad.PuzzleInputUsers[ii] = append(ad.PuzzleInputUsers[ii], puzzleInputData{
					User:     users[id],
					Progress: &ppd,
				})
			}

			return ad, p.Name, true
		}
	}

	return nil, "", false
}

func adminDataIndex(a *auth.Auth, event puzzles.Event) (*adminData, bool) {
	ad := &adminData{}

	users, err := a.ListUsers()
	if err != nil {
		panic(fmt.Errorf("list users: %w", err))
	}

	keys := slices.SortedFunc(maps.Keys(users), func(a, b string) int {
		return cmp.Compare(
			strings.ToLower(users[a].Name),
			strings.ToLower(users[b].Name),
		)
	})
	for _, id := range keys {
		user := users[id]
		ad.Users = append(ad.Users, user)
	}

	ad.Puzzles = event.Puzzles

	return ad, true
}

func adminMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		var ok bool
		var title string

		if user := r.PathValue("user"); user != "" {
			pd.Admin, title, ok = adminDataUser(a, event, user)
		} else if puzzle := r.PathValue("puzzle"); puzzle != "" {
			pd.Admin, title, ok = adminDataPuzzle(a, event, puzzle)
		} else {
			pd.Admin, ok = adminDataIndex(a, event)
		}

		if !ok {
			notFound.ServeHTTP(w, r)
			return
		}

		pd.Title = title

		next.ServeHTTP(w, r)
	})
}

func adminPuzzleInputHandler(event puzzles.Event, notfound http.Handler) http.Handler {
	handlers := make(map[string][]http.Handler, len(event.Puzzles))
	for _, puzzle := range event.Puzzles {
		handlers[puzzle.Path] = make([]http.Handler, len(puzzle.Inputs))
		for i, input := range puzzle.Inputs {
			handlers[puzzle.Path][i] = cachedHandler(input.Text, "text/plain; charset=utf-8")
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		puzzle, ok := handlers[r.PathValue("puzzle")]
		if !ok {
			notfound.ServeHTTP(w, r)
			return
		}

		idx, err := strconv.Atoi(r.PathValue("index"))
		if err != nil {
			notfound.ServeHTTP(w, r)
			return
		}

		if idx < 0 || idx >= len(puzzle) {
			notfound.ServeHTTP(w, r)
			return
		}

		puzzle[idx].ServeHTTP(w, r)
	})
}
