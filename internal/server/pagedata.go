package server

import (
	"cc/internal/ctxlog"
	"context"
	"net/http"
	"time"
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

	Puzzle      currentPuzzleData
	Puzzles     []puzzleData
	PuzzleAlign int

	User *userData

	Dark    bool
	Title   string
	Content contentData
}

type eventData struct {
	Path   string
	Name   string
	Solved int
	Total  int
}

type userData struct {
	Name        string
	Avatar      string
	InputOffset uint8
}

type contentData struct {
	Parts []partData
}

type partData struct {
	MD         string
	Answer     string
	WantAnswer bool
}

type currentPuzzleData struct {
	Path string
	Name string

	Part     int
	Anchor   string
	Timeout  time.Time
	Finished bool
}

type puzzleData struct {
	Path   string
	Name   string
	Solved int
	Locked bool
	Unlock *time.Time
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

		if pd.User != nil {
			ctxlog.AddExtra(ctx, "user", pd.User.Name)
		}
	})
}
