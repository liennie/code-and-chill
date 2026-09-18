package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/notifier"
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

// adminLastSolvesLimit caps the number of rows shown on the admin "Solves"
// page.
const adminLastSolvesLimit = 50

func adminLastSolvesData(a *auth.Auth, event puzzles.Event) []adminSolveData {
	_, solves, points := prepareSolves(a, event)

	puzzleByID := make(map[string]puzzles.Puzzle, len(event.Puzzles))
	for _, p := range event.Puzzles {
		puzzleByID[p.ID] = p
	}

	// Score per solve, in the same oldest-first order points are awarded.
	scores := make([]int, len(solves))
	for i, s := range solves {
		pk := pointKey{puzzle: s.puzzle, part: s.part}
		scores[i] = points[pk]
		points[pk]--
	}

	// solves is sorted oldest first; walk backwards for newest first.
	start := max(0, len(solves)-adminLastSolvesLimit)
	result := make([]adminSolveData, 0, len(solves)-start)
	for i := len(solves) - 1; i >= start; i-- {
		s := solves[i]
		puzzle, ok := puzzleByID[s.puzzle]
		if !ok {
			continue
		}

		result = append(result, adminSolveData{
			User:   s.progress.user,
			Puzzle: puzzle,
			Part:   s.part,
			Time:   s.time,
			Score:  scores[i],
		})
	}

	return result
}

func adminSolvesMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		pd.Admin = &adminData{
			LastSolves: adminLastSolvesData(a, event),
		}

		next.ServeHTTP(w, r)
	})
}

func adminUserListMiddleware(a *auth.Auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		pd.Admin = &adminData{}

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
			pd.Admin.Users = append(pd.Admin.Users, user)
		}

		next.ServeHTTP(w, r)
	})
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
		if _, ok := errors.AsType[*auth.UserNotFoundError](err); ok {
			return nil, "", false
		}
		panic(fmt.Errorf("user %q: %w", id, err))
	}

	ad.User = user
	ad.Progress = adminProgressData(a, event, id)

	return ad, user.Name, true
}

func adminUserMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		var ok bool
		var title string

		if user := r.PathValue("user"); user != "" {
			pd.Admin, title, ok = adminDataUser(a, event, user)
		}

		if !ok {
			notFound.ServeHTTP(w, r)
			return
		}

		pd.Title = title

		next.ServeHTTP(w, r)
	})
}

func adminPuzzleListMiddleware(event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Admin = &adminData{}
		pd.Admin.Puzzles = event.Puzzles

		next.ServeHTTP(w, r)
	})
}

func adminSlidesListMiddleware(decks []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Admin = &adminData{}
		pd.Admin.SlideDecks = decks

		next.ServeHTTP(w, r)
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
		pp := progress.Puzzles[puzzle.ID]
		for _, t := range pp.Parts {
			ppd.Solves = append(ppd.Solves, t.Time)
		}
		ppd.Incorrect = pp.Incorrect
	}

	return ppd
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

func adminPuzzleMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		var ok bool
		var title string

		if puzzle := r.PathValue("puzzle"); puzzle != "" {
			pd.Admin, title, ok = adminDataPuzzle(a, event, puzzle)
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
			_, handlers[puzzle.Path][i] = cachedHandler(input.Text, "text/plain; charset=utf-8")
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

type adminNotifierTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func adminNotifierTestHandler(notif *notifier.Notifier, pzls *puzzles.Puzzles, event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)

		if notif == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			enc.Encode(adminNotifierTestResponse{
				Error: "notifier is not configured",
			})
			return
		}

		path := r.PathValue("puzzle")
		var puzzle puzzles.Puzzle
		found := false
		if path == "__test__" {
			puzzle = puzzles.Puzzle{
				ID:     "test",
				Path:   "test",
				Name:   "Test Puzzle",
				Unlock: time.Now(),
				Parts: []puzzles.Part{{
					ID:   "1",
					Text: "# Test Puzzle\n\nThis is a notifier test triggered from the admin page. Ignore this message.\n",
				}},
			}
			found = true
		} else {
			for _, p := range event.Puzzles {
				if p.Path == path {
					puzzle = p
					found = true
					break
				}
			}
		}
		if !found {
			w.WriteHeader(http.StatusNotFound)
			enc.Encode(adminNotifierTestResponse{
				Error: fmt.Sprintf("puzzle %q not found", path),
			})
			return
		}

		err := notif.NotifyTest(r.Context(), pzls, event, puzzle)
		if err != nil {
			ctxlog.Get(r.Context()).Error("notifier test", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(adminNotifierTestResponse{
				Error: err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		enc.Encode(adminNotifierTestResponse{
			OK:      true,
			Message: fmt.Sprintf("Test notification sent for %q.", puzzle.Name),
		})
	})
}

type adminNotifierSetupResponse struct {
	OK                    bool   `json:"ok"`
	Message               string `json:"message,omitempty"`
	Error                 string `json:"error,omitempty"`
	NotificationsThreadID string `json:"notificationsThreadId,omitempty"`
	GeneralThreadID       string `json:"generalThreadId,omitempty"`
}

func adminNotifierSetupHandler(notif *notifier.Notifier, event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)

		if notif == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			enc.Encode(adminNotifierSetupResponse{
				Error: "notifier is not configured",
			})
			return
		}

		result, err := notif.Setup(r.Context(), event)
		if err != nil {
			ctxlog.Get(r.Context()).Error("notifier setup", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(adminNotifierSetupResponse{
				Error:                 err.Error(),
				NotificationsThreadID: result.NotificationsThreadID,
				GeneralThreadID:       result.GeneralThreadID,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		enc.Encode(adminNotifierSetupResponse{
			OK:                    true,
			Message:               fmt.Sprintf("Forum threads created for %q.", event.Name),
			NotificationsThreadID: result.NotificationsThreadID,
			GeneralThreadID:       result.GeneralThreadID,
		})
	})
}
