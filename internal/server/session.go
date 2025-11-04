package server

import (
	"net/http"
	"woc/internal/db"
)

func sessionMiddleware(db *db.DB, next http.Handler) http.Handler {
	// TODO session expiration

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.User = &userData{
			Name:   "Test",
			Avatar: "https://placedog.net/40/40",
		}

		next.ServeHTTP(w, r)
	})
}
