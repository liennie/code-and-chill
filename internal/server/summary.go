package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

type Summary struct {
	PreviousPuzzles []PuzzleSummary `json:"previousPuzzles"`
	CurrentPuzzle   *PuzzleSummary  `json:"currentPuzzle"`
	NextPuzzle      *PuzzleSummary  `json:"nextPuzzle"`
	Leaderboard     []UserSummary   `json:"leaderboard"`
	LastSolves      []SolveSummary  `json:"lastSolves"` // up to 5 last
}

type PuzzleSummary struct {
	Name       string              `json:"name"`
	UnlockTime time.Time           `json:"unlockTime"`
	Solvers    *[2]int             `json:"solvers"`    // number of solvers per part
	SolverList *[2][]SolverSummary `json:"solverList"` // per-part solver details
}

type SolverSummary struct {
	DiscordID string    `json:"discordId"`
	Time      time.Time `json:"time"`
}

type UserSummary struct {
	Name         string `json:"name"`
	DiscordID    string `json:"discordId"`
	Parts        int    `json:"parts"`
	Score        int    `json:"score"`
	Position     int    `json:"position"`
	PrevPosition int    `json:"prevPosition"` // rank at now() - 1h; 0 if not ranked then
	PosChange    int    `json:"posChange"`    // compared to now() - 1h
}

type SolveSummary struct {
	DiscordID  string    `json:"discordId"`
	PuzzleName string    `json:"puzzleName"`
	Part       int       `json:"part"`
	Score      int       `json:"points"`
	Time       time.Time `json:"time"`
}

const summaryLastSolves = 5

func summaryHandler(a *auth.Auth, event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := ctxlog.Get(r.Context())
		pd := pageDataFromContext(r.Context())
		now := pd.Now
		prevTime := now.Add(-time.Hour)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		discordIDs, err := a.Discord.UserIDs()
		if err != nil {
			logger.Error("get discord ids", "error", err)
			discordIDs = map[string]string{}
		}

		summary := Summary{
			PreviousPuzzles: []PuzzleSummary{},
			Leaderboard:     []UserSummary{},
			LastSolves:      []SolveSummary{},
		}

		// Current and next puzzles by unlock time.
		currentIdx := -1
		nextIdx := -1
		for i := range event.Puzzles {
			if !event.Puzzles[i].Unlock.After(now) {
				currentIdx = i
			} else if nextIdx == -1 {
				nextIdx = i
			}
		}

		// Snapshot at now.
		upsNow, solvesNow, pointsNow := prepareSolves(a, event)
		scoreOfSolve := make([]int, len(solvesNow))
		nowCount := 0
		for i, solve := range solvesNow {
			if solve.time.After(now) {
				break
			}
			up := solve.progress
			up.solved++
			pk := pointKey{puzzle: solve.puzzle, part: solve.part}
			score := pointsNow[pk]
			up.score += score
			pointsNow[pk]--
			if solve.time.After(up.lastSolve) {
				up.lastSolve = solve.time
			}
			scoreOfSolve[i] = score
			nowCount = i + 1
		}
		slices.SortFunc(upsNow, (*userProgress).Compare)

		buildUnlocked := func(p puzzles.Puzzle) PuzzleSummary {
			ps := PuzzleSummary{
				Name:       p.Name,
				UnlockTime: p.Unlock,
			}
			var solvers [2]int
			var solverList [2][]SolverSummary
			for _, up := range upsNow {
				pp := up.puzzles[p.ID]
				for j := range pp.Parts {
					if j >= len(solvers) {
						break
					}
					if pp.Parts[j].Time.After(now) {
						continue
					}
					solvers[j]++
					solverList[j] = append(solverList[j], SolverSummary{
						DiscordID: discordIDs[up.user.ID],
						Time:      pp.Parts[j].Time,
					})
				}
			}
			for j := range solverList {
				slices.SortFunc(solverList[j], func(a, b SolverSummary) int {
					return a.Time.Compare(b.Time)
				})
			}
			ps.Solvers = &solvers
			ps.SolverList = &solverList
			return ps
		}

		if currentIdx >= 0 {
			ps := buildUnlocked(event.Puzzles[currentIdx])
			summary.CurrentPuzzle = &ps
			for i := 0; i < currentIdx; i++ {
				summary.PreviousPuzzles = append(summary.PreviousPuzzles, buildUnlocked(event.Puzzles[i]))
			}
		}
		if nextIdx >= 0 {
			p := event.Puzzles[nextIdx]
			summary.NextPuzzle = &PuzzleSummary{
				Name:       p.Name,
				UnlockTime: p.Unlock,
			}
		}

		// Snapshot at prevTime for position change.
		upsPrev, solvesPrev, pointsPrev := prepareSolves(a, event)
		for _, solve := range solvesPrev {
			if solve.time.After(prevTime) {
				break
			}
			up := solve.progress
			up.solved++
			pk := pointKey{puzzle: solve.puzzle, part: solve.part}
			up.score += pointsPrev[pk]
			pointsPrev[pk]--
			if solve.time.After(up.lastSolve) {
				up.lastSolve = solve.time
			}
		}
		slices.SortFunc(upsPrev, (*userProgress).Compare)
		rankPrev := make(map[string]int, len(upsPrev))
		for i, up := range upsPrev {
			if up.solved == 0 {
				continue
			}
			rankPrev[up.user.ID] = i + 1
		}

		// Leaderboard entries.
		for i, up := range upsNow {
			currRank := i + 1
			prevRank := 0
			posChange := 0
			if prev, ok := rankPrev[up.user.ID]; ok {
				prevRank = prev
				posChange = prev - currRank
			}

			summary.Leaderboard = append(summary.Leaderboard, UserSummary{
				Name:         up.user.Name,
				DiscordID:    discordIDs[up.user.ID],
				Parts:        up.solved,
				Score:        up.score,
				Position:     currRank,
				PrevPosition: prevRank,
				PosChange:    posChange,
			})
		}

		// Last solves at-or-before now, newest first.
		puzzleName := make(map[string]string, len(event.Puzzles))
		for _, p := range event.Puzzles {
			puzzleName[p.ID] = p.Name
		}
		start := max(0, nowCount-summaryLastSolves)
		for i := nowCount - 1; i >= start; i-- {
			s := solvesNow[i]
			summary.LastSolves = append(summary.LastSolves, SolveSummary{
				DiscordID:  discordIDs[s.progress.user.ID],
				PuzzleName: puzzleName[s.puzzle],
				Part:       s.part + 1,
				Score:      scoreOfSolve[i],
				Time:       s.time,
			})
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			logger.Error("encode summary", "error", err)
		}
	})
}

var summaryDocBody = []byte(`{
  "description": "Event summary snapshot returned by /{event}/summary.json.",
  "fields": {
    "previousPuzzles": {
      "type": "PuzzleSummary[]",
      "description": "All puzzles that were unlocked before the current puzzle, in event order. Empty when no puzzle is unlocked or only the current one is."
    },
    "currentPuzzle": {
      "type": "PuzzleSummary | null",
      "description": "The most recently unlocked puzzle at request time. null when no puzzle is unlocked yet."
    },
    "nextPuzzle": {
      "type": "PuzzleSummary | null",
      "description": "The next puzzle to unlock after request time. null when all puzzles are already unlocked."
    },
    "leaderboard": {
      "type": "UserSummary[]",
      "description": "All ranked users, ordered by parts solved, score, and last solve time."
    },
    "lastSolves": {
      "type": "SolveSummary[]",
      "description": "The 5 most recent solves at or before request time. Newest solve first."
    }
  },
  "types": {
    "PuzzleSummary": {
      "name": {
        "type": "string",
        "description": "Display name of the puzzle."
      },
      "unlockTime": {
        "type": "string (RFC 3339 timestamp)",
        "description": "The time the puzzle unlocks for regular users."
      },
      "solvers": {
        "type": "[integer, integer] | null",
        "description": "Number of unique solvers for part 1 and part 2 at request time. null when the puzzle is not yet unlocked."
      },
      "solverList": {
        "type": "[SolverSummary[], SolverSummary[]] | null",
        "description": "Per-part list of solvers with Discord ID and solve time, ordered by solve time ascending. null when the puzzle is not yet unlocked."
      }
    },
    "SolverSummary": {
      "discordId": {
        "type": "string",
        "description": "Discord user ID (snowflake) of the solver. Empty string when the user has no linked Discord account."
      },
      "time": {
        "type": "string (RFC 3339 timestamp)",
        "description": "The time the solver completed this part."
      }
    },
    "UserSummary": {
      "name": {
        "type": "string",
        "description": "Display name of the user."
      },
      "discordId": {
        "type": "string",
        "description": "Discord user ID (snowflake) linked to this account. Empty string when the user has no linked Discord account."
      },
      "parts": {
        "type": "integer",
        "description": "Total number of puzzle parts the user has solved in this event."
      },
      "score": {
        "type": "integer",
        "description": "Total score the user has earned in this event."
      },
      "position": {
        "type": "integer",
        "description": "Current rank at request time, starting at 1."
      },
      "prevPosition": {
        "type": "integer",
        "description": "Rank one hour before the request, starting at 1. 0 when the user was not ranked one hour ago."
      },
      "posChange": {
        "type": "integer",
        "description": "Rank change compared to one hour before the request. Positive means the user moved up (better rank)."
      }
    },
    "SolveSummary": {
      "discordId": {
        "type": "string",
        "description": "Discord user ID (snowflake) linked to the solver's account. Empty string when the user has no linked Discord account."
      },
      "puzzleName": {
        "type": "string",
        "description": "Display name of the puzzle."
      },
      "part": {
        "type": "integer",
        "description": "The part number that was solved, starting at 1."
      },
      "points": {
        "type": "integer",
        "description": "Score awarded for this solve."
      },
      "time": {
        "type": "string (RFC 3339 timestamp)",
        "description": "The time the solve was recorded."
      }
    }
  }
}
`)

func summaryDocHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(summaryDocBody)
	})
}
