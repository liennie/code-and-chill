package server

import (
	"cmp"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"cc/internal/auth"
	"cc/internal/puzzles"
	"cc/internal/session"
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

		progress := progressFromContext(r.Context())

		pd.Puzzles = make([]puzzleData, 0, len(event.Puzzles))
		for _, puzzle := range event.Puzzles {
			pzd := puzzleData{
				Path:  puzzle.Path,
				Name:  puzzle.Name,
				Total: len(puzzle.Parts),
			}

			if puzzle.Unlock.After(pd.Now) {
				pzd.Locked = true
				pzd.Unlock = &puzzle.Unlock
			} else {
				pd.PuzzleUnlocked = true
			}

			if progress != nil {
				pzd.Solved = len(progress.Puzzles[puzzle.ID].Parts)
			}

			pd.Puzzles = append(pd.Puzzles, pzd)
		}

		next.ServeHTTP(w, r)
	})
}

func puzzleDataFunc(puzzle puzzles.Puzzle, locked dataFunc) dataFunc {
	return func(r *http.Request) (int, any) {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = currentPuzzleData{
			Path: puzzle.Path,
			Name: puzzle.Name,
		}
		pd.Title = puzzle.Name

		if puzzle.Unlock.After(pd.Now) {
			return locked(r)
		}

		sess := sessionFromContext(r.Context())
		user := sess.Data().User
		progress := progressFromContext(r.Context())
		pd.Content.Parts = make([]partData, 0, len(puzzle.Parts))
		for i, part := range puzzle.Parts {
			solved := false
			if progress != nil {
				solved = len(progress.Puzzles[puzzle.ID].Parts) > i
			}

			answer := ""
			if solved && user != nil {
				input := inputIndex(user, puzzle)
				answer = puzzle.Inputs[input].Answers[i]
			}

			pd.Content.Parts = append(pd.Content.Parts, partData{
				MD:         part.Text,
				Answer:     answer,
				WantAnswer: true,
			})

			if !solved {
				break
			}
		}
		if progress != nil {
			pd.Puzzle.Finished = len(progress.Puzzles[puzzle.ID].Parts) >= len(puzzle.Parts)
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

func inputIndex(user *session.User, puzzle puzzles.Puzzle) uint {
	h := fnv.New64()
	io.WriteString(h, user.ID)
	io.WriteString(h, puzzle.ID)
	return uint(h.Sum64()) % uint(len(puzzle.Inputs))
}

func puzzleInputHandler(puzzle puzzles.Puzzle, locked http.Handler, unauth http.Handler) http.Handler {
	handlers := make([]http.Handler, len(puzzle.Inputs))
	for i, input := range puzzle.Inputs {
		handlers[i] = cachedHandler(input.Text, "text/plain; charset=utf-8")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		if puzzle.Unlock.After(pd.Now) {
			locked.ServeHTTP(w, r)
			return
		}

		sess := sessionFromContext(r.Context())
		user := sess.Data().User
		if user == nil {
			// this should never happen
			unauth.ServeHTTP(w, r)
			return
		}

		i := inputIndex(user, puzzle)
		handlers[i].ServeHTTP(w, r)
	})
}

var errCancelUpdate = fmt.Errorf("cancel progress update")

type puzzleAnswerDataFuncs struct {
	locked        dataFunc
	unauth        dataFunc
	badRequest    dataFunc
	empty         dataFunc
	alreadySolved dataFunc
	badPart       dataFunc
	timeout       dataFunc
	incorrect     dataFunc
	correct       dataFunc
}

func puzzleAnswerDataFunc(a *auth.Auth, event puzzles.Event, pidx int, puzzle puzzles.Puzzle, dataFuncs puzzleAnswerDataFuncs) dataFunc {
	return func(r *http.Request) (int, any) {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = currentPuzzleData{
			Path: puzzle.Path,
			Name: puzzle.Name,
		}

		if puzzle.Unlock.After(pd.Now) {
			return dataFuncs.locked(r)
		}

		sess := sessionFromContext(r.Context())
		user := sess.Data().User
		if user == nil {
			// this should never happen
			return dataFuncs.unauth(r)
		}

		err := r.ParseForm()
		if err != nil {
			return dataFuncs.badRequest(r)
		}

		rawPart := r.PostForm.Get("part")
		part, err := strconv.Atoi(rawPart)
		if err != nil {
			return dataFuncs.badPart(r)
		}

		ii := inputIndex(user, puzzle)
		if part < 0 || part >= len(puzzle.Parts) || part >= len(puzzle.Inputs[ii].Answers) {
			return dataFuncs.badPart(r)
		}

		progress := progressFromContext(r.Context())
		if progress == nil && part != 0 {
			// quick check for first answer before we add progress with an update
			return dataFuncs.badPart(r)
		}

		var df dataFunc
		err = a.UpdateProgress(event.ID, user.ID, func(progress *auth.UserProgress) error {
			pp := progress.Puzzles[puzzle.ID]
			// second check with up to date data
			if part < len(pp.Parts) {
				part++
				if part < len(puzzle.Parts) {
					pd.Puzzle.Part = part + 1
					pd.Puzzle.Anchor = puzzle.Parts[part].ID
				} else {
					pd.Puzzle.Finished = true
				}
				df = dataFuncs.alreadySolved
				return errCancelUpdate

			} else if part > len(pp.Parts) {
				df = dataFuncs.badPart
				return errCancelUpdate
			}
			// part == len(pp.Parts)

			answer := r.PostForm.Get("answer")
			answer = strings.TrimSpace(answer)
			answer = strings.NewReplacer("\t", " ", "\n", " ", "\v", " ", "\f", " ", "\r", "").Replace(answer)
			if answer == "" {
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				df = dataFuncs.empty
				return errCancelUpdate
			}
			pd.Puzzle.Submitted = answer

			if progress.Timeout.After(pd.Now) {
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				pd.Puzzle.Timeout = progress.Timeout
				df = dataFuncs.timeout
				return errCancelUpdate
			}

			correctAnswer := puzzle.Inputs[ii].Answers[part]

			if answer != correctAnswer {
				progress.Incorrect++
				if progress.Incorrect < 5 {
					progress.Timeout = pd.Now.Add(time.Minute)
				} else {
					progress.Timeout = pd.Now.Add(5 * time.Minute)
				}

				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				df = dataFuncs.incorrect
				return nil
			}

			progress.Incorrect = 0
			progress.Timeout = time.Time{}
			// append is fine because we check that part == len(pp.Parts) above
			pp.Parts = append(pp.Parts, auth.PartProgress{
				Time: pd.Now,
			})
			progress.Puzzles[puzzle.ID] = pp

			if pidx >= 0 && pidx < len(pd.Puzzles) {
				pd.Puzzles[pidx].Solved++
			}

			part++
			if part < len(puzzle.Parts) {
				pd.Puzzle.Part = part + 1
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
			} else {
				pd.Puzzle.Finished = true
			}
			df = dataFuncs.correct
			return nil
		})
		if err != nil {
			if !errors.Is(err, errCancelUpdate) {
				panic(fmt.Errorf("update progress: %w", err))
			}
		}
		if df == nil {
			panic("no data func set in puzzle answer handler")
		}
		return df(r)
	}
}

type userProgress struct {
	User      *auth.User
	Puzzles   map[string]auth.PuzzleProgress
	Solved    int
	Score     int
	LastSolve time.Time
}

func (p *userProgress) Time(puzzleID string, part int) (time.Time, bool) {
	if puzzle, ok := p.Puzzles[puzzleID]; ok {
		if part < len(puzzle.Parts) {
			return puzzle.Parts[part].Time, true
		}
	}
	return time.Time{}, false
}

func (p *userProgress) ComparePart(other *userProgress, puzzleID string, part int) int {
	aTime, aOk := p.Time(puzzleID, part)
	bTime, bOk := other.Time(puzzleID, part)
	if aOk && bOk {
		return aTime.Compare(bTime)
	}
	if aOk {
		return -1
	}
	if bOk {
		return 1
	}
	return 0
}

func (p *userProgress) Compare(other *userProgress) int {
	return cmp.Or(
		-cmp.Compare(p.Solved, other.Solved),
		-cmp.Compare(p.Score, other.Score),
		p.LastSolve.Compare(other.LastSolve),
	)
}

func leaderboardMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		progress, err := a.AllProgress(event.ID)
		if err != nil {
			panic(fmt.Errorf("get all progress: %w", err))
		}

		ups := make([]*userProgress, 0, len(progress))
		for user, p := range progress {
			ups = append(ups, &userProgress{
				User:    user,
				Puzzles: p.Puzzles,
			})
		}

		for _, puzzle := range event.Puzzles {
			for part := range puzzle.Parts {
				slices.SortFunc(ups, func(a, b *userProgress) int {
					return a.ComparePart(b, puzzle.ID, part)
				})

				for i, up := range ups {
					t, ok := up.Time(puzzle.ID, part)
					if !ok {
						continue
					}

					up.Score += len(ups) - i
					up.Solved++
					if t.After(up.LastSolve) {
						up.LastSolve = t
					}
				}
			}
		}

		slices.SortFunc(ups, (*userProgress).Compare)
		if len(ups) > 50 {
			ups = ups[:50]
		}

		for _, up := range ups {
			pd.Leaderboard = append(pd.Leaderboard, leaderboardData{
				User: userData{
					ID:     up.User.ID,
					Name:   up.User.Name,
					Avatar: up.User.AvatarURL,
				},
				Solved: up.Solved,
				Score:  up.Score,
			})
		}

		next.ServeHTTP(w, r)
	})
}
