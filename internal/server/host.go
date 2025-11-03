package server

import (
	"net/http"
)

func hostMiddleware(host string, next http.Handler) http.Handler {
	if host == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Redirect(w, r, "//"+host+r.URL.RequestURI(), http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}
