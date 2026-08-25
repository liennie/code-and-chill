package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
)

// avatarMaxAge is the client-side cache lifetime for /avatar/{id} responses.
const avatarMaxAge = 24 * 60 * 60

func avatarHandler(a *auth.Auth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		avatar, err := a.UserAvatar(id)
		if err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("get user avatar", "id", id, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if avatar == nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("ETag", avatar.ETag)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", avatarMaxAge))

		if r.Header.Get("If-None-Match") == avatar.ETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if avatar.ContentType != "" {
			w.Header().Set("Content-Type", avatar.ContentType)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(avatar.Data)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(avatar.Data); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("write avatar response", "id", id, "error", err)
		}
	})
}
