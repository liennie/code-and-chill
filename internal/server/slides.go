package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/liennie/code-and-chill/internal/ctxlog"
)

// presentationData holds pre-calculated values for slide templates. Fields
// are flat, grouping related values into small substructs, so templates can
// use short paths such as .LB.First.Name.
type presentationData struct {
	Event presentationEventData
	Now   time.Time

	LB presentationLeaderboardData

	Solvers     int
	Part1Solves int
	Part2Solves int

	Fastest *presentationSolveData
	Slowest *presentationSolveData
}

type presentationEventData struct {
	Name string
}

type presentationSolveData struct {
	User   string
	Puzzle string
	Part   int
	Time   time.Time
	Unlock time.Time
}

type presentationLeaderboardData struct {
	First  *presentationPlaceData
	Second *presentationPlaceData
	Third  *presentationPlaceData
}

type presentationPlaceData struct {
	Name  string
	Parts int
	Score int
}

func newPresentationData(pd *pageData) presentationData {
	place := func(i int) *presentationPlaceData {
		if i >= len(pd.Leaderboard) || pd.Leaderboard[i].Solved == 0 {
			return nil
		}
		lb := pd.Leaderboard[i]
		return &presentationPlaceData{
			Name:  lb.User.Name,
			Parts: lb.Solved,
			Score: lb.Score,
		}
	}

	solve := func(s *solveExtremeData) *presentationSolveData {
		if s == nil {
			return nil
		}
		return &presentationSolveData{
			User:   s.User,
			Puzzle: s.Puzzle,
			Part:   s.Part,
			Time:   s.Time,
			Unlock: s.Unlock,
		}
	}

	return presentationData{
		Event: presentationEventData{
			Name: pd.Event.Name,
		},
		Now: pd.Now,
		LB: presentationLeaderboardData{
			First:  place(0),
			Second: place(1),
			Third:  place(2),
		},
		Solvers:     pd.Solvers,
		Part1Solves: pd.Part1Solves,
		Part2Solves: pd.Part2Solves,

		Fastest: solve(pd.FastestSolve),
		Slowest: solve(pd.SlowestSolve),
	}
}

const slidesDir = "html/slides"

// discoverSlideDecks lists the presentation decks under html/slides, one
// subdirectory per deck. A missing directory means no decks are configured.
func discoverSlideDecks(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, slidesDir)
	if err != nil {
		return nil
	}

	decks := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			decks = append(decks, e.Name())
		}
	}
	sort.Strings(decks)

	return decks
}

// slideDeck holds a deck's slide templates, parsed once but executed per
// request, in presentation order.
type slideDeck struct {
	templates []*template.Template
	backURL   string
}

// loadSlideDeck parses every slide file of a deck, in filename order.
func loadSlideDeck(fsys fs.FS, deck, backURL string) *slideDeck {
	dir := path.Join(slidesDir, deck)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		panic(fmt.Errorf("server: read slide deck %q: %w", deck, err))
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && path.Ext(e.Name()) == ".html" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	templates := make([]*template.Template, 0, len(names))
	for _, name := range names {
		content := readFile(fsys, path.Join(dir, name))
		t := template.Must(template.New(name).Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))
		templates = append(templates, t)
	}

	return &slideDeck{
		templates: templates,
		backURL:   backURL,
	}
}

// parseSlidesSkeleton parses the outer document that wraps a deck's rendered
// slides, e.g. templates/slides.html.
func parseSlidesSkeleton(fsys fs.FS) *template.Template {
	content := readFile(fsys, "templates/slides.html")
	return template.Must(template.New("slides").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))
}

// slidesPageData is the data passed to the slides.html skeleton.
type slidesPageData struct {
	Title  string
	Back   string
	ETags  map[string]string
	Slides []template.HTML
}

// handler renders every slide of the deck from one presentationData snapshot,
// so data such as the leaderboard cannot change mid-deck.
func (d *slideDeck) handler(skeleton *template.Template, title string, etags map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		data := newPresentationData(pd)

		slides := make([]template.HTML, 0, len(d.templates))
		for _, t := range d.templates {
			buf := &bytes.Buffer{}
			if err := t.Execute(buf, data); err != nil {
				logger := ctxlog.Get(r.Context())
				logger.Error("failed to exec slide", "error", err)
				panic(err)
			}
			slides = append(slides, template.HTML(buf.String()))
		}

		buf := &bytes.Buffer{}
		if err := skeleton.Execute(buf, slidesPageData{
			Title:  title,
			Back:   d.backURL,
			ETags:  etags,
			Slides: slides,
		}); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to exec slides page", "error", err)
			panic(err)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Allow same-origin framing so the admin deck list can show a live iframe thumbnail.
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		if _, err := io.Copy(w, buf); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to write response", "error", err)
		}
	})
}
