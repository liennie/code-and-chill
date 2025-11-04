package server

import (
	"net/http"
	"time"
	"woc/internal/ctxlog"
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
		l := ctxlog.Get(r.Context())
		l = l.With("method", r.Method, "url", r.URL.String(), "remote_addr", r.RemoteAddr)
		r = r.WithContext(ctxlog.Store(r.Context(), l))

		pd := pageDataFromContext(r.Context())

		cw := &statusCapturingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(cw, r)
		dur := time.Since(pd.Now)

		if pd.User != nil {
			l = l.With("user", pd.User.Name)
		}

		l.Info("request completed", "status", cw.status, "duration", dur.String())
	})
}
