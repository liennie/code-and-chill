package server

import (
	"context"
	"html/template"
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
	Now time.Time

	Event      eventData
	Events     []eventData
	EventAlign int

	Puzzle      int
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
	Name   string
	Avatar string
}

type contentData struct {
	Parts []partData
}

type partData struct {
	MD         string
	HTML       template.HTML
	Answer     string
	WantAnswer bool
}

type puzzleData struct {
	Name   string
	Class  string
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

func pageDataBaseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, pageDataKey, &pageData{
			Now: time.Now(),
		})
		r = r.Clone(ctx)
		next.ServeHTTP(w, r)
	})
}
