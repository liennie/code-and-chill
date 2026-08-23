package server

import (
	"archive/zip"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

func adminMux(admin http.Handler, notadmin http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())

		if user != nil && user.Admin {
			admin.ServeHTTP(w, r)
		} else {
			notadmin.ServeHTTP(w, r)
		}
	})
}

func adminUserListMiddleware(a *auth.Auth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		pd.Admin = &adminData{}

		users, err := a.ListUsers()
		if err != nil {
			panic(fmt.Errorf("list users: %w", err))
		}

		keys := slices.SortedFunc(maps.Keys(users), func(a, b string) int {
			return cmp.Compare(
				strings.ToLower(users[a].Name),
				strings.ToLower(users[b].Name),
			)
		})
		for _, id := range keys {
			user := users[id]
			pd.Admin.Users = append(pd.Admin.Users, user)
		}

		next.ServeHTTP(w, r)
	})
}

func adminProgressData(a *auth.Auth, event puzzles.Event, userID string) *progressData {
	progress, err := a.Progress(event.ID, userID)
	if err != nil {
		panic(fmt.Errorf("user %q progress: %w", userID, err))
	}
	pd := &progressData{}
	if progress != nil {
		pd.Incorrect = progress.Incorrect
		pd.Timeout = progress.Timeout
	}

	for _, puzzle := range event.Puzzles {
		pd.Puzzles = append(pd.Puzzles, adminPuzzleProgressData(puzzle, userID, progress))
	}

	return pd
}

func adminDataUser(a *auth.Auth, event puzzles.Event, id string) (*adminData, string, bool) {
	ad := &adminData{}

	user, err := a.User(id)
	if err != nil {
		if _, ok := errors.AsType[*auth.UserNotFoundError](err); ok {
			return nil, "", false
		}
		panic(fmt.Errorf("user %q: %w", id, err))
	}

	ad.User = user
	ad.Progress = adminProgressData(a, event, id)

	return ad, user.Name, true
}

func adminUserMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		var ok bool
		var title string

		if user := r.PathValue("user"); user != "" {
			pd.Admin, title, ok = adminDataUser(a, event, user)
		}

		if !ok {
			notFound.ServeHTTP(w, r)
			return
		}

		pd.Title = title

		next.ServeHTTP(w, r)
	})
}

func adminPuzzleListMiddleware(event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Admin = &adminData{}
		pd.Admin.Puzzles = event.Puzzles

		next.ServeHTTP(w, r)
	})
}

func adminPuzzleProgressData(puzzle puzzles.Puzzle, userID string, progress *auth.UserProgress) puzzleProgressData {
	ii := inputIndex(userID, puzzle)

	ppd := puzzleProgressData{
		Path:       puzzle.Path,
		Name:       puzzle.Name,
		Unlock:     puzzle.Unlock,
		Input:      puzzle.Inputs[ii].File,
		InputIndex: ii,
	}
	if progress != nil {
		for _, t := range progress.Puzzles[puzzle.ID].Parts {
			ppd.Solves = append(ppd.Solves, t.Time)
		}
	}

	return ppd
}

func adminDataPuzzle(a *auth.Auth, event puzzles.Event, puzzle string) (*adminData, string, bool) {
	for _, p := range event.Puzzles {
		if p.Path == puzzle {
			ad := &adminData{
				Puzzle:           &p,
				PuzzleInputUsers: make([][]puzzleInputData, len(p.Inputs)),
			}

			users, err := a.ListUsers()
			if err != nil {
				panic(fmt.Errorf("list users: %w", err))
			}

			keys := slices.SortedFunc(maps.Keys(users), func(a, b string) int {
				return cmp.Compare(
					strings.ToLower(users[a].Name),
					strings.ToLower(users[b].Name),
				)
			})
			for _, id := range keys {
				progress, err := a.Progress(event.ID, id)
				if err != nil {
					panic(fmt.Errorf("user %q progress: %w", id, err))
				}

				ii := inputIndex(id, p)
				ppd := adminPuzzleProgressData(p, id, progress)

				ad.PuzzleInputUsers[ii] = append(ad.PuzzleInputUsers[ii], puzzleInputData{
					User:     users[id],
					Progress: &ppd,
				})
			}

			return ad, p.Name, true
		}
	}

	return nil, "", false
}

func adminPuzzleMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		var ok bool
		var title string

		if puzzle := r.PathValue("puzzle"); puzzle != "" {
			pd.Admin, title, ok = adminDataPuzzle(a, event, puzzle)
		}

		if !ok {
			notFound.ServeHTTP(w, r)
			return
		}

		pd.Title = title

		next.ServeHTTP(w, r)
	})
}

func adminPuzzleInputHandler(event puzzles.Event, notfound http.Handler) http.Handler {
	handlers := make(map[string][]http.Handler, len(event.Puzzles))
	for _, puzzle := range event.Puzzles {
		handlers[puzzle.Path] = make([]http.Handler, len(puzzle.Inputs))
		for i, input := range puzzle.Inputs {
			handlers[puzzle.Path][i] = cachedHandler(input.Text, "text/plain; charset=utf-8")
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		puzzle, ok := handlers[r.PathValue("puzzle")]
		if !ok {
			notfound.ServeHTTP(w, r)
			return
		}

		idx, err := strconv.Atoi(r.PathValue("index"))
		if err != nil {
			notfound.ServeHTTP(w, r)
			return
		}

		if idx < 0 || idx >= len(puzzle) {
			notfound.ServeHTTP(w, r)
			return
		}

		puzzle[idx].ServeHTTP(w, r)
	})
}

type adminPresentationContainer struct {
	ret         string
	retHasQuery bool

	fileName    string
	contentType string
	content     []byte
}

func newAdminPresentationContainer(ret string) *adminPresentationContainer {
	return &adminPresentationContainer{
		ret:         ret,
		retHasQuery: strings.Contains(ret, "?"),
	}
}

func (a *adminPresentationContainer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Admin = &adminData{}

		if a.fileName != "" {
			pd.Admin.PresFileName = a.fileName
		}

		next.ServeHTTP(w, r)
	})
}

func (a *adminPresentationContainer) retURL(v url.Values) string {
	if len(v) == 0 {
		return a.ret
	}

	if a.retHasQuery {
		return a.ret + "&" + v.Encode()
	} else {
		return a.ret + "?" + v.Encode()
	}
}

func (a *adminPresentationContainer) retStatus(status string) string {
	if status == "" {
		return a.ret
	}

	v := url.Values{}
	v.Set("status", status)
	return a.retURL(v)
}

func (a *adminPresentationContainer) uploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := ctxlog.Get(r.Context())

		ret := func(status string) {
			http.Redirect(w, r, a.retStatus(status), http.StatusSeeOther)
		}

		err := r.ParseMultipartForm(100 * 1024 * 1024)
		if err != nil {
			logger.Error("multipart parse", "error", err)
			ret("parse-err")
			return
		}

		presFiles := r.MultipartForm.File["presentation"]
		if len(presFiles) == 0 {
			ret("no-file")
			return
		}
		if len(presFiles) != 1 {
			ret("too-many-files")
			return
		}

		fileMeta := presFiles[0]
		f, err := fileMeta.Open()
		if err != nil {
			logger.Error("multipart open", "error", err)
			ret("open-err")
			return
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			logger.Error("multipart read", "error", err)
			ret("read-err")
			return
		}

		_, err = zip.NewReader(bytes.NewReader(content), fileMeta.Size)
		if err != nil {
			logger.Error("multipart unzip", "error", err)
			ret("zip-err")
			return
		}

		a.fileName = fileMeta.Filename
		a.contentType = fileMeta.Header.Get("Content-Type")
		a.content = content
		ret("up-ok")
	})
}

func (a *adminPresentationContainer) downloadHandler(notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.fileName == "" {
			notFound.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", a.contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", a.fileName))
		w.WriteHeader(http.StatusOK)
		w.Write(a.content)
	})
}

func (a *adminPresentationContainer) renderHandler(notFound http.Handler) http.Handler {
	readAll := func(f *zip.File) ([]byte, error) {
		r, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open: %w", err)
		}
		defer r.Close()

		return io.ReadAll(r)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.fileName == "" {
			notFound.ServeHTTP(w, r)
			return
		}

		srcZip, err := zip.NewReader(bytes.NewReader(a.content), int64(len(a.content)))
		if err != nil {
			panic(fmt.Errorf("zip reader: %w", err))
		}

		pd := pageDataFromContext(r.Context())

		dst := &bytes.Buffer{}
		dstZip := zip.NewWriter(dst)
		for _, f := range srcZip.File {
			if f.Name == "content.xml" {
				content, err := readAll(f)
				if err != nil {
					panic(fmt.Errorf("read %q: %w", f.Name, err))
				}

				t, err := template.New(f.Name).Funcs(sprig.HtmlFuncMap()).Funcs(extraFuncs).Parse(string(content))
				if err != nil {
					panic(fmt.Errorf("parse %q: %w", f.Name, err))
				}

				fh := f.FileHeader
				w, err := dstZip.CreateHeader(&fh)
				if err != nil {
					panic(fmt.Errorf("create %q: %w", f.Name, err))
				}

				err = t.Execute(w, pd)
				if err != nil {
					panic(fmt.Errorf("execute %q: %w", f.Name, err))
				}

			} else {
				err := dstZip.Copy(f)
				if err != nil {
					panic(fmt.Errorf("copy %q: %w", f.Name, err))
				}
			}
		}

		err = dstZip.Close()
		if err != nil {
			panic(fmt.Errorf("close zip: %w", err))
		}

		// pd := pageDataFromContext(r.Context())
		// t.Execute()

		w.Header().Set("Content-Type", a.contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", a.fileName))
		w.WriteHeader(http.StatusOK)
		w.Write(dst.Bytes())
	})
}

func (a *adminPresentationContainer) clearHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.fileName = ""
		a.content = nil
		http.Redirect(w, r, a.retStatus("clear-ok"), http.StatusSeeOther)
	})
}
