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
	"path"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"cc/internal/ctxlog"
	"cc/internal/mdext"

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

func dataFile(fsys fs.FS, file string) ([]byte, string) {
	content := readFile(fsys, file)
	ct := mime.TypeByExtension(path.Ext(file))
	return content, ct
}

func cachedHandler(content []byte, ct string) http.Handler {
	h := md5.New()
	h.Write(content)
	etag := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			log := ctxlog.Get(r.Context())
			log.Error("failed to write response", "error", err)
			return
		}
	})
}

var extraFuncs = template.FuncMap{
	"rfc3339Time": func(t time.Time) string {
		return t.Format(time.RFC3339)
	},
	"formatDuration": func(d time.Duration) string {
		d = d.Round(time.Second)
		return strings.NewReplacer("h", "h ", "m", "m ").Replace(d.String())
	},
	"renderMD": func(md string) (template.HTML, error) {
		gm := goldmark.New(
			goldmark.WithExtensions(
				extension.Strikethrough,
				extension.Table,
				&mdext.InlineAttrExtender{},
				&mdext.RawStringExtender{},
				&mdext.SpanExtender{},
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
				parser.WithAttribute(),
			),
			goldmark.WithRendererOptions(
				html.WithHardWraps(),
			),
		)
		var buf bytes.Buffer
		if err := gm.Convert([]byte(md), &buf); err != nil {
			return "", fmt.Errorf("markdown: %w", err)
		}
		return template.HTML(buf.String()), nil
	},
	"puzzleHint": func(puzzle puzzleData) string {
		return strings.Repeat("✔", puzzle.Solved)
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
	"mdesc": mdext.RawStringEscape,
}

type dataFunc func(r *http.Request) (int, any)

func templateHandler(content []byte, ct string) func(dataFunc) http.Handler {
	t := template.Must(template.New("page").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))

	return func(dataFunc dataFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, data := dataFunc(r)

			buf := &bytes.Buffer{}
			if err := t.Execute(buf, data); err != nil {
				log := ctxlog.Get(r.Context())
				log.Error("failed to exec template", "error", err)
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
		pd.Title = title

		buf := &strings.Builder{}
		if err := t.Execute(buf, pd); err != nil {
			log := ctxlog.Get(r.Context())
			log.Error("failed to exec md", "error", err)
			panic(err)
		}

		pd.Content.Parts = []partData{{
			MD: buf.String(),
		}}
		return status, pd
	}
}
