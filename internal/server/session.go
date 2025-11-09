package server

import (
	"context"
	"fmt"
	"net/http"

	"cc/internal/session"
)

type sessionCtxKey struct{}

var sessionKey sessionCtxKey

func sessionIDFromContext(ctx context.Context) string {
	data, ok := ctx.Value(sessionKey).(string)
	if !ok {
		panic("session missing in context")
	}
	return data
}

func sessionID(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func sessionMiddleware(session *session.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		ID := sessionID(r)
		sess, err := session.Init(ID, pd.Now)
		if err != nil {
			panic(fmt.Errorf("get session: %w", err))
		}

		if sess.Update {
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    sess.ID,
				Expires:  sess.Expire,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, sessionKey, sess.ID)
		r = r.WithContext(ctx)

		// TODO users in a different middleware
		pd.User = &userData{
			Name:   "Test User",
			Avatar: "https://placedog.net/40/40",
		}

		next.ServeHTTP(w, r)
	})
}
