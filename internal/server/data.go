package server

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	ttemplate "text/template"

	"github.com/liennie/code-and-chill/internal/chrono"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/mdext"

	"github.com/Masterminds/sprig/v3"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

func readFile(fsys fs.FS, file string) []byte {
	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		panic(fmt.Errorf("server: read data file %q: %w", file, err))
	}
	return content
}

var contentType = map[string]string{
	".css": "text/css; charset=utf-8",
	".ico": "image/x-icon",
	".js":  "text/javascript; charset=utf-8",
	".svg": "image/svg+xml; charset=utf-8",
	".ttf": "font/ttf",
	".txt": "text/plain; charset=utf-8",
}

func dataFile(fsys fs.FS, file string) ([]byte, string) {
	content := readFile(fsys, file)

	ext := path.Ext(file)
	if ct, ok := contentType[ext]; ok {
		return content, ct
	}
	return content, mime.TypeByExtension(ext)
}

var maxAge = map[string]int{
	"font/ttf":                       14 * 24 * 60 * 60,
	"image/svg+xml; charset=utf-8":   14 * 24 * 60 * 60,
	"image/x-icon":                   14 * 24 * 60 * 60,
	"text/css; charset=utf-8":        1 * 24 * 60 * 60,
	"text/javascript; charset=utf-8": 1 * 24 * 60 * 60,
}

func cachedHandler(content []byte, ct string) http.Handler {
	h := md5.New()
	h.Write(content)
	etag := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct != "" {
			w.Header().Set("Content-Type", ct)

			if maxAge, ok := maxAge[ct]; ok {
				w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", maxAge))
			}
		}
		w.Header().Set("ETag", etag)

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			w.Write([]byte{})
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to write response", "error", err)
			return
		}
	})
}

var extraFuncs = template.FuncMap{
	"renderMD": func(md string) (template.HTML, error) {
		return renderMarkdown(md)
	},

	"rfc3339Time":    chrono.RFC3339Time,
	"humanTime":      chrono.HumanTime,
	"unlockTime":     chrono.UnlockTime,
	"formatDuration": chrono.FormatDuration,
	"solveTime":      chrono.SolveTime,

	"puzzleClass": func(puzzle puzzleData) string {
		if puzzle.Locked {
			return puzzleClassLocked
		}
		if puzzle.Solved == 1 {
			return puzzleClassSolvedOne
		}
		if puzzle.Solved >= 2 {
			return puzzleClassSolvedBoth
		}
		return puzzleClassUnlocked
	},

	"pad": func(digits int, n any) (string, error) {
		switch n.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:

			return fmt.Sprintf("%0[1]*d", digits, n), nil
		}
		return "", fmt.Errorf("cannot pad type %T", n)
	},

	"queryEsc": url.QueryEscape,
	"htmlEsc":  html.EscapeString,

	"choose": func(cond bool, t, f any) any {
		if cond {
			return t
		}
		return f
	},

	"openTag": func(tag string) template.HTML {
		return template.HTML("<" + tag)
	},

	"closeTag": func(tag string) template.HTML {
		return template.HTML("</" + tag + ">")
	},

	"cleanUTF": func(s string) string {
		return strings.ToValidUTF8(s, "")
	},

	"tabIndent": func(tabs int, v template.HTML) template.HTML {
		pad := strings.Repeat("\t", tabs)
		return template.HTML("\n" + pad + strings.ReplaceAll(strings.TrimSpace(string(v)), "\n", "\n"+pad))
	},

	"incorrectMsg": func(puzzle currentPuzzleData) string {
		const notCorrect = "not correct"

		if puzzle.Incorrect > 2 && rand.IntN(3) != 0 {
			return notCorrect
		}

		correct, err := strconv.Atoi(puzzle.Correct)
		if err != nil {
			return notCorrect
		}

		submitted, err := strconv.Atoi(puzzle.Submitted)
		if err != nil {
			return notCorrect
		}

		if submitted > (correct + rand.IntN(9000) + 1000) {
			return "too high"
		}

		if submitted < (correct - rand.IntN(9000) - 1000) {
			return "too low"
		}

		return notCorrect
	},

	"spreadQDots": func(s string) string {
		return strings.ReplaceAll(s, "::", " :: ")
	},

	"icon": func(s string) template.HTML {
		u, err := url.Parse(s)
		if err != nil {
			return ""
		}

		switch u.Host {
		case "discord.com":
			return `<i class="discord icon"></i>`
		case "github.com":
			return `<i class="github icon"></i>`
		}

		return ""
	},
}

type dataFunc func(r *http.Request) int

func renderMarkdown(md string) (template.HTML, error) {
	gm := goldmark.New(
		goldmark.WithExtensions(
			extension.Strikethrough,
			extension.Table,
			&mdext.InlineAttrExtender{},
			&mdext.SpanExtender{},
			&mdext.CodeSpanExtender{},
			&mdext.CodeBlockExtender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(),
	)
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return "", fmt.Errorf("markdown: %w", err)
	}
	return template.HTML(buf.String()), nil
}

func templateHandler(content []byte, ct string) func(dataFunc) http.Handler {
	t := template.Must(template.New("page").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))

	return func(dataFunc dataFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := dataFunc(r)
			pd := pageDataFromContext(r.Context())

			buf := &bytes.Buffer{}
			if err := t.Execute(buf, pd); err != nil {
				logger := ctxlog.Get(r.Context())
				logger.Error("failed to exec template", "error", err)
				panic(err)
			}

			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			io.Copy(w, buf)
		})
	}
}

func mdDataFunc(status int, title string, content []byte) dataFunc {
	t, err := ttemplate.New("md").Funcs(sprig.TxtFuncMap()).Funcs(extraFuncs).Parse(string(content))
	if err != nil {
		panic(fmt.Errorf("title %q: %w", title, err))
	}

	return func(r *http.Request) int {
		pd := pageDataFromContext(r.Context())
		if pd.Title != "" {
			pd.Title = title + " :: " + pd.Title
		} else {
			pd.Title = title
		}

		buf := &strings.Builder{}
		if err := t.Execute(buf, pd); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to exec md", "error", err)
			panic(err)
		}

		html, err := renderMarkdown(buf.String())
		if err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to render md", "error", err)
			panic(err)
		}
		pd.Content = html
		return status
	}
}

func htmlDataFunc(status int, title string, content []byte) dataFunc {
	t, err := template.New("html").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content))
	if err != nil {
		panic(fmt.Errorf("title %q: %w", title, err))
	}

	return func(r *http.Request) int {
		pd := pageDataFromContext(r.Context())
		if title != "" {
			if pd.Title != "" {
				pd.Title = title + " :: " + pd.Title
			} else {
				pd.Title = title
			}
		}

		buf := &strings.Builder{}
		if err := t.Execute(buf, pd); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to exec html", "error", err)
			panic(err)
		}

		pd.Content = template.HTML(buf.String())
		return status
	}
}
