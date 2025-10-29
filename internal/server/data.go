package server

import (
	"context"
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
)

func dataFile(fsys fs.FS, file string) ([]byte, string) {
	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		panic(fmt.Errorf("server: read data file %q: %w", file, err))
	}

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
	"renderMD": func(md string) string {
		return md // TODO
	},
}

func templateHandler(content []byte, ct string) func(func(ctx context.Context) (int, any)) http.Handler {
	t := template.Must(template.New("page").Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content)))

	return func(dataFunc func(ctx context.Context) (int, any)) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, data := dataFunc(r.Context())
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			if err := t.Execute(w, data); err != nil {
				log := ctxlog.Get(r.Context())
				log.Error("failed to write response", "error", err)
				return
			}
			// TODO handle errors
			// TODO handle custom status codes
		})
	}
}

func notFoundHandler(content []byte, ct string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write(content); err != nil {
			log := ctxlog.Get(r.Context())
			log.Error("failed to write response", "error", err)
			return
		}
	})
}

func internalServerErrorHandler(content []byte, ct string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write(content); err != nil {
			log := ctxlog.Get(r.Context())
			log.Error("failed to write response", "error", err)
			return
		}
	})
}

type userData struct {
	Name   string
	Avatar string
}

type partData struct {
	MD         string
	Answer     string
	WantAnswer bool
}

type contentData struct {
	Parts []partData
}

const (
	puzzleClassLocked     = "locked"
	puzzleClassUnlocked   = "unlocked"
	puzzleClassSolvedOne  = "solved-one"
	puzzleClassSolvedBoth = "solved-both"
)

type puzzleData struct {
	Name   string
	Class  string
	Unlock *time.Time
}

type pageData struct {
	Title   string
	Year    int
	Puzzle  int
	User    *userData
	Content contentData
	Puzzles []puzzleData
}
