package server

import (
	"net/http"
	"net/url"
	"time"

	"cc/internal/ctxlog"
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

		l := ctxlog.Get(ctx)
		l = l.With("method", r.Method, "url", u.String())

		ctx = ctxlog.Store(r.Context(), l)
		ctx = ctxlog.WithExtra(ctx)
		r = r.WithContext(ctx)

		cw := &statusCapturingResponseWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(cw, r)
		dur := time.Since(start)

		l = l.With("status", cw.status)
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
				l = l.With("location", loc.String())
			}
		}

		l = l.With(ctxlog.GetExtra(ctx)...)

		l.Info("request completed", "remote_addr", r.RemoteAddr, "duration", dur.String())
	})
}
