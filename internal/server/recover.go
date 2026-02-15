package server

import (
	"io"
	"net/http"
	"runtime"
	"runtime/debug"

	"cc/internal/ctxlog"
)

func recoverMiddleware(next, err http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				logger := ctxlog.Get(r.Context())
				if _, runtimeErr := e.(runtime.Error); runtimeErr {
					logger.Error("recovered panic", "error", e, "stack", string(debug.Stack()))
				} else {
					logger.Error("recovered panic", "error", e)
				}

				clear(w.Header())
				err.ServeHTTP(w, r)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

const catchAllHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<title>500: Internal Server Error</title>
</head>

<body>
<h1>500: Internal Server Error</h1>
<p>Something went very wrong on our end.</p>
<p>If this keeps happening, please <a href="/contact">let us know</a> what you were doing when the error appeared.</p>
</body>
</html>
`

func catchAllHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, catchAllHTML)
	})
}
