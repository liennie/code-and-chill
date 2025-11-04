package server

import (
	"net/http"
)

func darkModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		cookie, err := r.Cookie("darkmode")
		if err != nil || cookie.Value != "0" {
			pd.Dark = true
		}
		next.ServeHTTP(w, r)
	})
}
