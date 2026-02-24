package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cc/internal/auth"
)

type APIError struct {
	Err string `json:"error"`
}

func (e *APIError) Error() string {
	return e.Err
}

type APIListUsersResponse struct {
	Users map[string]*auth.User `json:"users"`
}

type APIGetUserResponse struct {
	User *auth.User `json:"user"`
}

type APIUpdateUserRequest struct {
	Name         *string `json:"name"`
	AvatarURL    *string `json:"avatar_url"`
	RandomAvatar *bool   `json:"random_avatar"`
	Token        *string `json:"token"`
	Admin        *bool   `json:"admin"`
	Hidden       *bool   `json:"hidden"`
}

type APIUpdateUserResponse struct {
	User *auth.User `json:"user"`
}

func apiHandler(auth *auth.Auth) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/", apiNotFound())

	mux.Handle("GET /users", apiListUsers(auth))
	mux.Handle("GET /user/{id}", apiGetUser(auth))
	mux.Handle("POST /user/{id}", apiUpdateUser(auth))

	// TODO progress handlers

	handler := http.Handler(mux)
	handler = recoverMiddleware(handler, apiCatchAllHandler())
	handler = logMiddleware(handler)

	return handler
}

func apiCatchAllHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":"Recovered panic. See server log for details."}`)
	})
}

func apiNotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"not found"}`)
	})
}

func apiListUsers(a *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		listFunc := a.ListUsers
		if name := r.URL.Query().Get("name"); name != "" {
			listFunc = func() (map[string]*auth.User, error) {
				return a.FindUsers(func(u *auth.User) bool {
					return strings.Contains(strings.ToLower(u.Name), strings.ToLower(name))
				})
			}
		}

		enc := json.NewEncoder(w)
		users, err := listFunc()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(APIError{
				Err: fmt.Errorf("list users: %w", err).Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		enc.Encode(APIListUsersResponse{
			Users: users,
		})
	})
}

func apiGetUser(a *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		enc := json.NewEncoder(w)
		user, err := a.User(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(APIError{
				Err: fmt.Errorf("get user: %w", err).Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		enc.Encode(APIGetUserResponse{
			User: user,
		})
	})
}

func apiUpdateUser(a *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		enc := json.NewEncoder(w)

		var req APIUpdateUserRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		err := dec.Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(APIError{
				Err: fmt.Errorf("request: %w", err).Error(),
			})
			return
		}

		var user *auth.User
		err = a.UpdateUser(r.PathValue("id"), func(u *auth.User) error {
			if req.Name != nil {
				u.Name = *req.Name
			}
			if req.AvatarURL != nil {
				u.AvatarURL = *req.AvatarURL
			}
			if req.RandomAvatar != nil {
				u.RandomAvatar = *req.RandomAvatar
			}
			if req.Token != nil {
				u.Token = *req.Token
			}
			if req.Admin != nil {
				u.Admin = *req.Admin
			}
			if req.Hidden != nil {
				u.Hidden = *req.Hidden
			}

			user = u
			return nil
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(APIError{
				Err: fmt.Errorf("update user: %w", err).Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		enc.Encode(APIUpdateUserResponse{
			User: user,
		})
	})
}
