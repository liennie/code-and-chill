package server

import (
	"context"
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
		sess := session.Get(ID, pd.Now)

		if sess.ID != ID {
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    sess.ID,
				Expires:  pd.Now.Add(session.Expire),
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, sessionKey, &sess)
		r = r.WithContext(ctx)

		// TODO users in a different middleware
		pd.User = &userData{
			Name:   "Test User",
			Avatar: "https://placedog.net/40/40",
		}

		next.ServeHTTP(w, r)

		session.Update(sess)
	})
}
