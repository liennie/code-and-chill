package server

import (
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
				State: state,
				Event: event.Path,
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

		var state, event string
		redirect := func() {
			if event == "" {
				http.Redirect(w, r, "/login/fail", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/"+event+"/login/fail", http.StatusSeeOther)
			}
		}

		err := sess.Update(func(data *session.Data) error {
			if data.Auth == nil {
				return fmt.Errorf("no auth state in session")
			}

			state = data.Auth.State
			event = data.Auth.Event
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

		// TODO update db
		// TODO update session
		_ = user

		http.Redirect(w, r, "/"+event, http.StatusSeeOther)
	})
}
