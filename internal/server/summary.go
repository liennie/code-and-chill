package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

type Summary struct {
	CurrentPuzzle *PuzzleSummary `json:"currentPuzzle"`
	NextPuzzle    *PuzzleSummary `json:"nextPuzzle"`
	Leaderboard   []UserSummary  `json:"leaderboard"` // up to 5 top
	LastSolves    []SolveSummary `json:"lastSolves"`  // up to 5 last
}

type PuzzleSummary struct {
	Name       string    `json:"name"`
	UnlockTime time.Time `json:"unlockTime"`
	Solvers    *[2]int   `json:"solvers"` // number of solvers per part
}

type UserSummary struct {
	Name              string `json:"name"`
	DiscordID         string `json:"discordId"`
	AvatarData        string `json:"avatarData"`
	AvatarContentType string `json:"avatarContentType"`
	Parts             int    `json:"parts"`
	Score             int    `json:"score"`
	PosChange         int    `json:"posChange"` // compared to now() - 1h
}

type SolveSummary struct {
	UserName   string    `json:"userName"`
	DiscordID  string    `json:"discordId"`
	PuzzleName string    `json:"puzzleName"`
	Part       int       `json:"part"`
	Score      int       `json:"points"`
	Time       time.Time `json:"time"`
}

const summaryTopN = 5

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
			Leaderboard: []UserSummary{},
			LastSolves:  []SolveSummary{},
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

		if currentIdx >= 0 {
			p := event.Puzzles[currentIdx]
			ps := &PuzzleSummary{
				Name:       p.Name,
				UnlockTime: p.Unlock,
			}
			var solvers [2]int
			for _, up := range upsNow {
				pp := up.puzzles[p.ID]
				for j := range pp.Parts {
					if j >= len(solvers) {
						break
					}
					if !pp.Parts[j].Time.After(now) {
						solvers[j]++
					}
				}
			}
			ps.Solvers = &solvers
			summary.CurrentPuzzle = ps
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

		// Top leaderboard entries.
		top := upsNow
		if len(top) > summaryTopN {
			top = top[:summaryTopN]
		}
		for i, up := range top {
			currRank := i + 1
			posChange := 0
			if prev, ok := rankPrev[up.user.ID]; ok {
				posChange = prev - currRank
			}

			var avatarData, avatarContentType string
			av, err := a.UserAvatar(up.user.ID)
			if err != nil {
				logger.Error("get user avatar", "id", up.user.ID, "error", err)
			} else if av != nil && len(av.Data) > 0 {
				avatarData = base64.StdEncoding.EncodeToString(av.Data)
				avatarContentType = av.ContentType
			}

			summary.Leaderboard = append(summary.Leaderboard, UserSummary{
				Name:              up.user.Name,
				DiscordID:         discordIDs[up.user.ID],
				AvatarData:        avatarData,
				AvatarContentType: avatarContentType,
				Parts:             up.solved,
				Score:             up.score,
				PosChange:         posChange,
			})
		}

		// Last solves at-or-before now, newest first.
		puzzleName := make(map[string]string, len(event.Puzzles))
		for _, p := range event.Puzzles {
			puzzleName[p.ID] = p.Name
		}
		start := max(0, nowCount-summaryTopN)
		for i := nowCount - 1; i >= start; i-- {
			s := solvesNow[i]
			summary.LastSolves = append(summary.LastSolves, SolveSummary{
				UserName:   s.progress.user.Name,
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
      "description": "The top 5 users, ranked by parts solved, score, and last solve time."
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
      "avatarData": {
        "type": "string (base64)",
        "description": "Base64-encoded avatar image bytes. Empty string when no cached avatar is available."
      },
      "avatarContentType": {
        "type": "string",
        "description": "MIME content type for avatarData. Empty string when avatarData is empty."
      },
      "parts": {
        "type": "integer",
        "description": "Total number of puzzle parts the user has solved in this event."
      },
      "score": {
        "type": "integer",
        "description": "Total score the user has earned in this event."
      },
      "posChange": {
        "type": "integer",
        "description": "Rank change compared to one hour before the request. Positive means the user moved up (better rank)."
      }
    },
    "SolveSummary": {
      "userName": {
        "type": "string",
        "description": "Display name of the user who made the solve."
      },
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
