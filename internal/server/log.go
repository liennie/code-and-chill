package server

import (
	"net/http"
	"net/url"
	"time"

	"github.com/liennie/code-and-chill/internal/ctxlog"
)

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusCapturingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		u := *r.URL // copy
		if u.RawQuery != "" {
			u.RawQuery = "REDACTED"
		}

		logger := ctxlog.Get(ctx)
		logger = logger.With("method", r.Method, "url", u.String())

		ctx = ctxlog.Store(r.Context(), logger)
		ctx = ctxlog.WithExtra(ctx)
		r = r.WithContext(ctx)

		cw := &statusCapturingResponseWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(cw, r)
		dur := time.Since(start)

		logger = logger.With("status", cw.status)
		switch cw.status {
		case http.StatusMovedPermanently,
			http.StatusFound,
			http.StatusSeeOther,
			http.StatusTemporaryRedirect,
			http.StatusPermanentRedirect:

			loc, err := url.Parse(cw.Header().Get("Location"))
			if err == nil {
				if loc.RawQuery != "" {
					loc.RawQuery = "REDACTED"
				}
				logger = logger.With("location", loc.String())
			}
		}

		logger = logger.With(ctxlog.GetExtra(ctx)...)

		logger.Info("request completed", "remote_addr", r.RemoteAddr, "duration", dur.String())
	})
}
