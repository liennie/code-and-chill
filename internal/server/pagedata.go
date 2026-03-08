package server

import (
	"context"
	"net/http"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

const (
	puzzleClassLocked     = "locked"
	puzzleClassUnlocked   = "unlocked"
	puzzleClassSolvedOne  = "solved-one"
	puzzleClassSolvedBoth = "solved-both"
)

type pageData struct {
	Request *http.Request

	Name string
	Now  time.Time

	Event      eventData
	Events     []eventData
	EventAlign int

	Puzzle         currentPuzzleData
	Puzzles        []puzzleData
	PuzzleAlign    int
	PuzzleUnlocked bool

	User *userData

	Leaderboard []leaderboardData

	Dark    bool
	Title   string
	Content contentData

	Admin *adminData
}

type eventData struct {
	Path   string
	Name   string
	Solved int
	Total  int
}

type currentPuzzleData struct {
	Path string
	Name string

	Submitted string
	Part      int
	Anchor    string
	Timeout   time.Time
	Finished  bool
}

type puzzleData struct {
	Path   string
	Name   string
	Solved int
	Total  int
	Locked bool
	Unlock *time.Time
}

type userData struct {
	ID     string
	Name   string
	Avatar string
	Admin  bool
	Hidden bool
}

type leaderboardData struct {
	User   userData
	Solved int
	Score  int
}

type contentData struct {
	Parts []partData
}

type partData struct {
	Anchor     string
	MD         string
	Answer     string
	WantAnswer bool
}

type adminData struct {
	Users   []*auth.User
	Puzzles []puzzles.Puzzle

	User     *auth.User
	Progress *progressData

	Puzzle           *puzzles.Puzzle
	PuzzleInputUsers [][]puzzleInputData
}

type puzzleInputData struct {
	User     *auth.User
	Progress *puzzleProgressData
}

type progressData struct {
	Incorrect uint
	Timeout   time.Time
	Puzzles   []puzzleProgressData
}

type puzzleProgressData struct {
	Path       string
	Name       string
	Unlock     time.Time
	Input      string
	InputIndex uint
	Solves     []time.Time
}

type pageDataCtxKey struct{}

var pageDataKey pageDataCtxKey

func pageDataFromContext(ctx context.Context) *pageData {
	data, ok := ctx.Value(pageDataKey).(*pageData)
	if !ok {
		panic("page data missing in context")
	}
	return data
}

func pageDataMiddleware(name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := &pageData{
			Request: r,
			Name:    name,
			Now:     time.Now(),
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, pageDataKey, pd)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
