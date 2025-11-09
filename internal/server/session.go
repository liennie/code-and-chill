package server

import (
	"context"
	"fmt"
	"net/http"

	"cc/internal/session"
)

type sessionCtxKey struct{}

var sessionKey sessionCtxKey

func sessionFromContext(ctx context.Context) *session.Session {
	data, ok := ctx.Value(sessionKey).(*session.Session)
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

		sid := sess.ID()
		if sid.Update {
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    sid.ID,
				Path:     "/",
				Expires:  sid.Expire,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, sessionKey, sess)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
