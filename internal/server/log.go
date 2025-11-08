package server

import (
	"net/http"
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

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		l := ctxlog.Get(ctx)
		l = l.With("method", r.Method, "url", r.URL.String(), "remote_addr", r.RemoteAddr)

		ctx = ctxlog.Store(r.Context(), l)
		ctx = ctxlog.WithExtra(ctx)
		r = r.WithContext(ctx)

		cw := &statusCapturingResponseWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(cw, r)
		dur := time.Since(start)

		l = l.With(ctxlog.GetExtra(ctx)...)

		l.Info("request completed", "status", cw.status, "duration", dur.String())
	})
}
