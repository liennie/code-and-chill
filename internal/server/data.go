package server

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"woc/internal/ctxlog"
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

func tooManyRequestsHandler(content []byte, ct string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.WriteHeader(http.StatusTooManyRequests)
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
