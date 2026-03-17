package server

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/mdext"

	"github.com/Masterminds/sprig/v3"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to write response", "error", err)
			return
		}
	})
}

var extraFuncs = template.FuncMap{
	"rfc3339Time": func(t time.Time) string {
		return t.Format(time.RFC3339)
	},
	"humanTime": func(t time.Time) string {
		if t.Second() == 0 {
			return t.Format("Mon, Jan 02, 15:04")
		}
		return t.Format("Mon, Jan 02, 15:04:05")
	},
	"formatDuration": func(d time.Duration) string {
		d = d.Round(time.Second)

		dd := d / (24 * time.Hour)
		d = d % (24 * time.Hour)

		hh := d / (time.Hour)
		d = d % (time.Hour)

		mm := d / (time.Minute)
		d = d % (time.Minute)

		ss := d / (time.Second)

		switch {
		case dd > 0:
			return fmt.Sprintf("%dd %02dh %02dm %02ds", dd, hh, mm, ss)

		case hh > 0:
			return fmt.Sprintf("%dh %02dm %02ds", hh, mm, ss)

		case mm > 0:
			return fmt.Sprintf("%dm %02ds", mm, ss)

		default:
			return fmt.Sprintf("%ds", ss)
		}
	},
	"renderMD": func(md string) (template.HTML, error) {
		gm := goldmark.New(
			goldmark.WithExtensions(
				extension.Strikethrough,
				extension.Table,
				&mdext.InlineAttrExtender{},
				&mdext.RawStringExtender{},
				&mdext.SpanExtender{},
				&mdext.CheckboxExtender{},
				&mdext.CodeSpanExtender{},
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
				parser.WithAttribute(),
			),
			goldmark.WithRendererOptions(
				html.WithHardWraps(),
				html.WithUnsafe(),
			),
		)
		var buf bytes.Buffer
		if err := gm.Convert([]byte(md), &buf); err != nil {
			return "", fmt.Errorf("markdown: %w", err)
		}
		return template.HTML(buf.String()), nil
	},
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
	"mdesc":    mdext.RawStringEscape,
	"codeesc":  mdext.CodeStringEscape,
	"queryesc": url.QueryEscape,
	"choose": func(cond bool, t, f any) any {
		if cond {
			return t
		}
		return f
	},
	"opentag": func(tag string) template.HTML {
		return template.HTML("<" + tag)
	},
	"closetag": func(tag string) template.HTML {
		return template.HTML("</" + tag + ">")
	},
	"cleanutf": func(s string) string {
		return strings.ToValidUTF8(s, "")
	},
}

type dataFunc func(r *http.Request) (int, any)

func templateHandler(content []byte, ct string) func(dataFunc) http.Handler {
	t := template.Must(template.New("page").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))

	return func(dataFunc dataFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, data := dataFunc(r)

			buf := &bytes.Buffer{}
			if err := t.Execute(buf, data); err != nil {
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

	return func(r *http.Request) (int, any) {
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

		pd.Content.Parts = []partData{{
			MD: buf.String(),
		}}
		return status, pd
	}
}
