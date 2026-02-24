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
		errRedirect := func() {
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
		redirect := func() {
			redir := "/" + event
			if ret != "" {
				redir += "/" + ret
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
			logger := ctxlog.Get(r.Context())
			logger.Error("auth session", "error", err)
			errRedirect()
			return
		}

		query := r.URL.Query()

		returnedState := query.Get("state")
		if returnedState != state {
			logger := ctxlog.Get(r.Context())
			logger.Error("auth state", "error", err)
			errRedirect()
			return
		}

		if query.Get("error") == "access_denied" {
			logger := ctxlog.Get(r.Context())
			logger.Info("auth canceled by user")
			redirect()
			return
		}

		token := rand.Text()

		code := query.Get("code")
		user, err := auth.Discord.Exchange(r.Context(), code, token)
		if err != nil {
			logger := ctxlog.Get(r.Context())
			if extra, ok := ctxlog.ErrExtra(err); ok {
				logger = logger.With("extra", extra)
			}
			logger.Error("exchange", "error", err)
			errRedirect()
			return
		}

		err = sess.Update(func(data *session.Data) error {
			data.User = &session.User{
				ID:    user.ID,
				Token: token,
			}
			return nil
		})
		if err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("update session", "error", err)
			errRedirect()
			return
		}

		redirect()
	})
}

type userCtxKey struct{}

var userKey userCtxKey

func userFromContext(ctx context.Context) *auth.User {
	data, _ := ctx.Value(userKey).(*auth.User)
	return data
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

			if user.Token != su.Token {
				err := sess.Update(func(data *session.Data) error {
					data.User = nil
					return nil
				})
				if err != nil {
					panic(fmt.Errorf("update session: %w", err))
				}
			} else {
				pd.User = &userData{
					ID:     user.ID,
					Name:   user.Name,
					Avatar: user.AvatarURL,
					Admin:  user.Admin,
				}

				progress, err := a.Progress(event.ID, su.ID)
				if err != nil {
					panic(fmt.Errorf("get progress: %w", err))
				}

				ctx := r.Context()
				ctx = context.WithValue(ctx, progressKey, progress)
				ctx = context.WithValue(ctx, userKey, user)

				ctxlog.AddExtra(ctx, "user", user.Name)

				r = r.WithContext(ctx)
			}
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
			logger := ctxlog.Get(r.Context())
			logger.Error("logout", "error", err)
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
		user := userFromContext(r.Context())

		if user != nil {
			loggedin.ServeHTTP(w, r)
		} else {
			loggedout.ServeHTTP(w, r)
		}
	})
}
