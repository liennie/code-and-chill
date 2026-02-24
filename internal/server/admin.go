package server

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"cc/internal/auth"
	"cc/internal/puzzles"
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

func adminDataUser(a *auth.Auth, event puzzles.Event, id string) *adminData {
	ad := &adminData{}

	user, err := a.User(id)
	if err != nil {
		panic(fmt.Errorf("user %q: %w", id, err))
	}

	ad.User = user

	progress, err := a.Progress(event.ID, id)
	if err != nil {
		panic(fmt.Errorf("user %q progress: %w", id, err))
	}

	ad.Progress = &progressData{
		Incorrect: progress.Incorrect,
		Timeout:   progress.Timeout,
	}
	for _, puzzle := range event.Puzzles {
		ii := inputIndex(id, puzzle)
		ppd := puzzleProgressData{
			Path:       puzzle.Path,
			Name:       puzzle.Name,
			Unlock:     puzzle.Unlock,
			Input:      puzzle.Inputs[ii].File,
			InputIndex: ii,
		}
		for _, t := range progress.Puzzles[puzzle.ID].Parts {
			ppd.Solves = append(ppd.Solves, t.Time)
		}

		ad.Progress.Puzzles = append(ad.Progress.Puzzles, ppd)
	}

	return ad
}

func adminDataIndex(a *auth.Auth) *adminData {
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

	return ad
}

func adminMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		if user := r.PathValue("user"); user != "" {
			pd.Admin = adminDataUser(a, event, user)
		} else {
			pd.Admin = adminDataIndex(a)
		}

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
