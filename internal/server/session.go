package server

import (
	"net/http"
	"woc/internal/db"
)

func sessionMiddleware(db *db.DB, next http.Handler) http.Handler {
	// TODO session expiration

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}
