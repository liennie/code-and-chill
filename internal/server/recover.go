package server

import (
	"net/http"
	"woc/internal/ctxlog"
)

type rech struct {
	next http.Handler
	err  http.Handler
}

func newRecover(next, err http.Handler) *rech {
	return &rech{
		next: next,
		err:  err,
	}
}

func (rec *rech) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log := ctxlog.Get(r.Context())
			log.Error("recovered panic", "error", err)

			clear(w.Header())
			rec.err.ServeHTTP(w, r)
		}
	}()

	rec.next.ServeHTTP(w, r)
}
