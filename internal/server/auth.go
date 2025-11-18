package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	"cc/internal/auth"
	"cc/internal/ctxlog"
	"cc/internal/puzzles"
	"cc/internal/session"
)

func discordAuthRedirect(auth *auth.Auth, event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := rand.Text()

		sess := sessionFromContext(r.Context())
		err := sess.Update(func(data *session.Data) error {
			data.Auth = &session.Auth{
				State:  state,
				Event:  event.Path,
				Return: r.URL.Query().Get("return"),
			}
			return nil
		})
		if err != nil {
			panic(fmt.Errorf("update session: %w", err))
		}

		http.Redirect(w, r, auth.Discord.AuthURL(state), http.StatusSeeOther)
	})
}

func discordAuthCallback(auth *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromContext(r.Context())

		var state, event, ret string
		redirect := func() {
			var redir string

			if event == "" {
				redir = "/login/fail"
			} else {
				redir = "/" + event + "/login/fail"
			}

			if ret != "" {
				redir += "?return=" + ret
			}

			http.Redirect(w, r, redir, http.StatusSeeOther)
		}

		err := sess.Update(func(data *session.Data) error {
			if data.Auth == nil {
				return fmt.Errorf("no auth state in session")
			}

			state = data.Auth.State
			event = data.Auth.Event
			ret = data.Auth.Return
			data.Auth = nil

			return nil
		})
		if err != nil {
			l := ctxlog.Get(r.Context())
			l.Error("auth session", "error", err)
			redirect()
			return
		}

		returnedState := r.URL.Query().Get("state")
		if returnedState != state {
			l := ctxlog.Get(r.Context())
			l.Error("auth state", "error", err)
			redirect()
			return
		}

		code := r.URL.Query().Get("code")
		user, err := auth.Discord.Exchange(r.Context(), code)
		if err != nil {
			l := ctxlog.Get(r.Context())
			if extra, ok := ctxlog.ErrExtra(err); ok {
				l = l.With("extra", extra)
			}
			l.Error("exchange", "error", err)
			redirect()
			return
		}

		err = sess.Update(func(data *session.Data) error {
			data.User = &session.User{ID: user.ID}
			return nil
		})
		if err != nil {
			l := ctxlog.Get(r.Context())
			l.Error("update session", "error", err)
			redirect()
			return
		}

		redir := "/" + event
		if ret != "" {
			redir += "/" + ret
		}
		http.Redirect(w, r, redir, http.StatusSeeOther)
	})
}

type progressCtxKey struct{}

var progressKey progressCtxKey

func progressFromContext(ctx context.Context) *auth.UserProgress {
	data, _ := ctx.Value(progressKey).(*auth.UserProgress)
	return data
}

func userMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		sess := sessionFromContext(r.Context())

		su := sess.Data().User
		if su != nil {
			user, err := a.User(su.ID)
			if err != nil {
				panic(fmt.Errorf("get user: %w", err))
			}

			pd.User = &userData{
				Name:        user.Name,
				Avatar:      user.AvatarURL,
				InputOffset: user.InputOffset,
			}

			progress, err := a.Progress(event.ID, su.ID)
			if err != nil {
				panic(fmt.Errorf("get progress: %w", err))
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, progressKey, progress)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func logoutHandler(event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromContext(r.Context())

		err := sess.Update(func(data *session.Data) error {
			data.User = nil
			return nil
		})
		if err != nil {
			l := ctxlog.Get(r.Context())
			l.Error("logout", "error", err)
		}

		redir := "/" + event.Path
		if ret := r.URL.Query().Get("return"); ret != "" {
			redir += "/" + ret
		}
		http.Redirect(w, r, redir, http.StatusSeeOther)
	})
}

func userMux(loggedin http.Handler, loggedout http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromContext(r.Context())

		if sess.Data().User != nil {
			loggedin.ServeHTTP(w, r)
		} else {
			loggedout.ServeHTTP(w, r)
		}
	})
}
