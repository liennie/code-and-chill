package server

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"time"
	"woc/internal/ctxlog"

	"github.com/Masterminds/sprig/v3"
	"github.com/yuin/goldmark"
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
	"renderMD": func(md string) (template.HTML, error) {
		gm := goldmark.New(
			// goldmark.WithExtensions(
			// 	extension.Strikethrough,
			// ),
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
}

type dataFunc func(r *http.Request) (int, any)

func templateHandler(content []byte, ct string) func(dataFunc) http.Handler {
	t := template.Must(template.New("page").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))

	return func(dataFunc dataFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, data := dataFunc(r)
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			if err := t.Execute(w, data); err != nil {
				log := ctxlog.Get(r.Context())
				log.Error("failed to write response", "error", err)
				return
			}
		})
	}
}

func mdDataFunc(status int, title string, content []byte) dataFunc {
	return func(r *http.Request) (int, any) {
		pd := pageDataFromContext(r.Context())
		pd.Title = title
		pd.Content.Parts = []partData{{
			MD:         string(content),
			WantAnswer: false,
		}}
		return status, pd
	}
}
