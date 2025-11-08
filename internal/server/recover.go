package server

import (
	"net/http"

	"cc/internal/ctxlog"
)

func recoverMiddleware(next, err http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log := ctxlog.Get(r.Context())
				log.Error("recovered panic", "error", e)

				clear(w.Header())
				err.ServeHTTP(w, r)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func catchAllHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Something went very wrong", http.StatusInternalServerError)
	})
}
