// Package server implements the web server, routes and middleware for the
// code-and-chill application.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/db"
	"github.com/liennie/code-and-chill/internal/notifier"
	"github.com/liennie/code-and-chill/internal/puzzles"
	"github.com/liennie/code-and-chill/internal/session"

	"golang.org/x/sync/errgroup"
)

type Server struct {
	addr            string
	handler         http.Handler
	tlsLoader       *tlsLoader
	httpsRedirect   bool
	shutdownTimeout time.Duration
	apiPort         int
	apiHandler      http.Handler
}

func New(config Config, db *db.DB, session *session.Store, auth *auth.Auth, puzzles *puzzles.Puzzles, notif *notifier.Notifier) *Server {
	if config.Port == 0 {
		panic("server: port is required")
	}
	if config.DataDir == "" {
		panic("server: dataDir is required")
	}
	if config.ShutdownTimeout == 0 {
		panic("server: shutdownTimeout is required")
	}

	var loader *tlsLoader
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		loader = newTLSLoader(config.TLSCertFile, config.TLSKeyFile, config.TLSReloadSchedule)
	} else if config.TLSCertFile != "" || config.TLSKeyFile != "" {
		panic("server: both tlsCertFile and tlsKeyFile must be set to enable TLS")
	}

	return &Server{
		addr:            net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
		handler:         newHandler(config, db, session, auth, puzzles, notif),
		tlsLoader:       loader,
		httpsRedirect:   config.HTTPSRedirect,
		shutdownTimeout: config.ShutdownTimeout,
		apiPort:         config.APIPort,
		apiHandler:      apiMiddleware(apiHandler(auth)),
	}
}

func (s *Server) Close() error {
	if s.tlsLoader != nil {
		err := s.tlsLoader.Close()
		s.tlsLoader = nil
		return err
	}
	return nil
}

func handlerRegistrar(prefix string, mux *http.ServeMux) func(method, path, src string, handler http.Handler) {
	return func(method, path, src string, handler http.Handler) {
		slog.Info("registering handler", "method", method, "path", prefix+path, "src", src)
		if method != "" {
			path = method + " " + path
		}
		mux.Handle(path, handler)
	}
}

func newHandler(config Config, db *db.DB, session *session.Store, auth *auth.Auth, pzls *puzzles.Puzzles, notif *notifier.Notifier) (h http.Handler) {
	fsys := os.DirFS(filepath.FromSlash(config.DataDir))

	// handlers
	mux := http.NewServeMux()
	reg := handlerRegistrar("", mux)

	page := templateHandler(dataFile(fsys, "templates/page.html"))
	badRequestDataFunc := htmlDataFunc(http.StatusBadRequest, "400: Bad Request", readFile(fsys, "html/400.html"))
	unauthorizedDataFunc := htmlDataFunc(http.StatusUnauthorized, "401: Unauthorized", readFile(fsys, "html/401.html"))
	unauthorizedHandler := page(unauthorizedDataFunc)
	notFoundHandler := page(htmlDataFunc(http.StatusNotFound, "404: Not Found", readFile(fsys, "html/404.html")))
	internalErrorHandler := page(htmlDataFunc(http.StatusInternalServerError, "500: Internal Server Error", readFile(fsys, "html/500.html")))

	eventMiddleware := func(event puzzles.Event) func(http.Handler) http.Handler {
		return func(handler http.Handler) http.Handler {
			handler = puzzlesMiddleware(event, handler)
			handler = darkModeMiddleware(handler)
			handler = userMiddleware(auth, event, handler)
			handler = sessionMiddleware(session, handler)
			handler = recoverMiddleware(handler, internalErrorHandler)
			handler = pageDataMiddleware(pzls.Name, handler)
			return handler
		}
	}
	rootMiddleware := eventMiddleware(pzls.Default)

	// static
	const wwwDir = "www"
	err := fs.WalkDir(fsys, wwwDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		subPath, ok := strings.CutPrefix(p, wwwDir)
		if !ok {
			panic(fmt.Errorf("%q is not a subpath of %q", p, wwwDir))
		}

		reg("GET", subPath, p, cachedHandler(dataFile(fsys, p)))
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("server: walk www directory: %w", err))
	}

	// root
	e := pzls.Default.Path
	reg("", "/", "html/404.html", rootMiddleware(notFoundHandler))
	reg("GET", "/{$}", "redirect", http.RedirectHandler("/"+e, http.StatusTemporaryRedirect))
	// reg("GET", "/events", "redirect", http.RedirectHandler("/"+e+"/events", http.StatusTemporaryRedirect))
	reg("GET", "/rules", "redirect", http.RedirectHandler("/"+e+"/rules", http.StatusTemporaryRedirect))
	reg("GET", "/leaderboard", "redirect", http.RedirectHandler("/"+e+"/leaderboard", http.StatusTemporaryRedirect))
	reg("GET", "/contact", "redirect", http.RedirectHandler("/"+e+"/contact", http.StatusTemporaryRedirect))
	reg("GET", "/latest", "redirect", http.RedirectHandler("/"+e+"/latest", http.StatusTemporaryRedirect))

	// reg("GET", "/login", "redirect", http.RedirectHandler("/"+e+"/login", http.StatusTemporaryRedirect))
	reg("GET", "/login/fail", "redirect", http.RedirectHandler("/"+e+"/login/fail", http.StatusTemporaryRedirect))
	reg("GET", "/profile", "redirect", http.RedirectHandler("/"+e+"/profile", http.StatusTemporaryRedirect))
	reg("GET", "/logout", "redirect", http.RedirectHandler("/"+e+"/logout", http.StatusTemporaryRedirect))

	reg("GET", "/auth/discord", "discordAuthCallback", rootMiddleware(userMux(
		http.RedirectHandler("/", http.StatusSeeOther),
		discordAuthCallback(auth)),
	))

	reg("GET", "/avatar/{id}", "avatarHandler", avatarHandler(auth))

	reg("GET", "/admin.js", "extra/admin.js", rootMiddleware(adminMux(
		cachedHandler(dataFile(fsys, "extra/admin.js")),
		notFoundHandler,
	)))

	reg("", "/api", "apiHandler", rootMiddleware(adminMux(
		apiNotFound(),
		notFoundHandler,
	)))

	reg("", "/api/", "apiHandler", rootMiddleware(adminMux(
		http.StripPrefix("/api", apiHandler(auth)),
		notFoundHandler,
	)))

	// events
	lockedDataFunc := htmlDataFunc(http.StatusNotFound, "Puzzle locked", readFile(fsys, "html/puzzle/locked.html"))
	puzzleContentDataFunc := htmlDataFunc(http.StatusOK, "", readFile(fsys, "html/page/puzzle.html"))
	for _, event := range pzls.Events {
		e = event.Path

		evMux := http.NewServeMux()
		middleware := eventMiddleware(event)
		reg("GET", "/"+e+"/", "mux", http.StripPrefix("/"+e, middleware(evMux)))
		reg("POST", "/"+e+"/", "mux", http.StripPrefix("/"+e, middleware(evMux)))
		reg("GET", "/"+e, "html/page/home.html", http.StripPrefix("/"+e, middleware(page(htmlDataFunc(http.StatusOK, "", readFile(fsys, "html/page/home.html"))))))

		reg := handlerRegistrar("/"+e, evMux)

		reg("GET", "/", "html/404.html", notFoundHandler)
		// reg("GET", "/events", "html/page/events.html", eventsMiddleware(pzls.Events, page(htmlDataFunc(http.StatusOK, "Events", readFile(fsys, "html/page/events.html")))))
		reg("GET", "/rules", "html/page/rules.html", page(htmlDataFunc(http.StatusOK, "Rules", readFile(fsys, "html/page/rules.html"))))
		reg("GET", "/leaderboard", "html/page/leaderboard.html", leaderboardMiddleware(auth, event, page(htmlDataFunc(http.StatusOK, "Leaderboard", readFile(fsys, "html/page/leaderboard.html")))))
		reg("GET", "/leaderboard/chart.svg", "leaderboardChart", leaderboardChart(auth, event))
		reg("GET", "/contact", "html/page/contact.html", page(htmlDataFunc(http.StatusOK, "Contact", readFile(fsys, "html/page/contact.html"))))
		reg("GET", "/latest", "latestPuzzleRedirect", latestPuzzleRedirect(event))

		// reg("GET", "/login", "html/page/login.html", userMux(
		// 	returnRedirect(e),
		// 	page(htmlDataFunc(http.StatusOK, "Log In", readFile(fsys, "html/page/login.html"))),
		// ))
		reg("GET", "/login/discord", "discordAuthHandler", userMux(
			returnRedirect(event),
			discordAuthRedirect(auth, event),
		))
		reg("GET", "/login/fail", "html/page/loginfail.html", userMux(
			returnRedirect(event),
			page(htmlDataFunc(http.StatusUnauthorized, "Login failed", readFile(fsys, "html/page/loginfail.html"))),
		))
		reg("GET", "/profile", "html/page/profile.html", userMux(
			page(htmlDataFunc(http.StatusOK, "Profile", readFile(fsys, "html/page/profile.html"))),
			http.RedirectHandler("/"+e+"/login/discord?return=profile", http.StatusSeeOther),
		))
		reg("POST", "/logout", "logoutHandler", userMux(
			logoutHandler(event),
			returnRedirect(event),
		))

		for pidx, puzzle := range event.Puzzles {
			p := puzzle.Path

			reg("GET", "/puzzle/"+p, "puzzleDataFunc", page(puzzleDataFunc(puzzle, lockedDataFunc, puzzleContentDataFunc)))
			reg("GET", "/puzzle/"+p+"/input", "puzzleInputHandler", userMux(
				puzzleInputHandler(puzzle, page(lockedDataFunc), unauthorizedHandler),
				unauthorizedHandler,
			))
			reg("GET", "/puzzle/"+p+"/answer", "redirect", http.RedirectHandler("/"+e+"/puzzle/"+p, http.StatusSeeOther))
			reg("POST", "/puzzle/"+p+"/answer", "puzzleAnswerHandler", userMux(
				page(puzzleAnswerDataFunc(auth, event, pidx, puzzle, puzzleAnswerDataFuncs{
					locked:        lockedDataFunc,
					unauth:        unauthorizedDataFunc,
					badRequest:    badRequestDataFunc,
					empty:         htmlDataFunc(http.StatusBadRequest, puzzle.Name, readFile(fsys, "html/puzzle/empty.html")),
					alreadySolved: htmlDataFunc(http.StatusBadRequest, puzzle.Name, readFile(fsys, "html/puzzle/alreadysolved.html")),
					badPart:       htmlDataFunc(http.StatusBadRequest, puzzle.Name, readFile(fsys, "html/puzzle/badpart.html")),
					timeout:       htmlDataFunc(http.StatusTooManyRequests, puzzle.Name, readFile(fsys, "html/puzzle/timeout.html")),
					incorrect:     htmlDataFunc(http.StatusOK, puzzle.Name, readFile(fsys, "html/puzzle/incorrect.html")),
					correct:       htmlDataFunc(http.StatusOK, puzzle.Name, readFile(fsys, "html/puzzle/correct.html")),
				})),
				unauthorizedHandler,
			))
		}

		// admin
		reg("GET", "/admin", "html/admin/admin.html", adminMux(
			page(htmlDataFunc(http.StatusOK, "Admin", readFile(fsys, "html/admin/admin.html"))),
			notFoundHandler,
		))

		reg("GET", "/admin/users", "html/admin/users.html", adminMux(
			adminUserListMiddleware(auth, page(htmlDataFunc(http.StatusOK, "Admin :: Users", readFile(fsys, "html/admin/users.html")))),
			notFoundHandler,
		))

		reg("GET", "/admin/user/{user}", "html/admin/user.html", adminMux(
			adminUserMiddleware(auth, event, page(htmlDataFunc(http.StatusOK, "Admin :: Users", readFile(fsys, "html/admin/user.html"))), notFoundHandler),
			notFoundHandler,
		))

		reg("GET", "/admin/puzzles", "html/admin/puzzles.html", adminMux(
			adminPuzzleListMiddleware(event, page(htmlDataFunc(http.StatusOK, "Admin :: Puzzles", readFile(fsys, "html/admin/puzzles.html")))),
			notFoundHandler,
		))

		reg("GET", "/admin/puzzle/{puzzle}", "html/admin/puzzle.html", adminMux(
			adminPuzzleMiddleware(auth, event, page(htmlDataFunc(http.StatusOK, "Admin :: Puzzles", readFile(fsys, "html/admin/puzzle.html"))), notFoundHandler),
			notFoundHandler,
		))

		reg("GET", "/admin/puzzle/{puzzle}/input/{index}", "adminPuzzleInputHandler", adminMux(
			adminPuzzleInputHandler(event, notFoundHandler),
			notFoundHandler,
		))

		pres := newAdminPresentationContainer("/" + e + "/admin/presentation")

		reg("GET", "/admin/presentation", "html/admin/presentation.html", adminMux(
			pres.middleware(page(htmlDataFunc(http.StatusOK, "Admin :: Presentation", readFile(fsys, "html/admin/presentation.html")))),
			notFoundHandler,
		))

		reg("POST", "/admin/presentation/upload", "adminPresentationContainer.uploadHandler", adminMux(
			pres.uploadHandler(),
			notFoundHandler,
		))

		reg("GET", "/admin/presentation/download", "adminPresentationContainer.downloadHandler", adminMux(
			pres.downloadHandler(notFoundHandler),
			notFoundHandler,
		))

		reg("GET", "/admin/presentation/render", "adminPresentationContainer.renderHandler", adminMux(
			leaderboardMiddleware(auth, event, pres.renderHandler(notFoundHandler)),
			notFoundHandler,
		))

		reg("POST", "/admin/presentation/clear", "adminPresentationContainer.clearHandler", adminMux(
			pres.clearHandler(),
			notFoundHandler,
		))

		reg("GET", "/admin/notifier", "html/admin/notifier.html", adminMux(
			adminPuzzleListMiddleware(event, page(htmlDataFunc(http.StatusOK, "Admin :: Notifier", readFile(fsys, "html/admin/notifier.html")))),
			notFoundHandler,
		))

		reg("POST", "/admin/notifier/test/{puzzle}", "adminNotifierTestHandler", adminMux(
			adminNotifierTestHandler(notif, pzls, event),
			notFoundHandler,
		))
	}

	// global middleware
	handler := http.Handler(mux)
	handler = headersMiddleware(handler)
	handler = hostMiddleware(config.Host, handler)
	handler = recoverMiddleware(handler, catchAllHandler())
	handler = logMiddleware(handler)

	return handler
}

func (s *Server) runServer(ctx context.Context, cancel context.CancelFunc, srv *http.Server) error {
	logger := ctxlog.Get(ctx)

	serveErrCh := make(chan error, 1)
	go func() {
		defer cancel()
		logger.Info("server is running", "addr", srv.Addr)
		if srv.TLSConfig != nil {
			serveErrCh <- srv.ListenAndServeTLS("", "")
		} else {
			serveErrCh <- srv.ListenAndServe()
		}
	}()

	<-ctx.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	shutdownErr := srv.Shutdown(stopCtx)
	stopCancel()

	<-stopCtx.Done()
	if errors.Is(stopCtx.Err(), context.DeadlineExceeded) {
		logger.Error("server shutdown timeout exceeded")

	} else if errors.Is(stopCtx.Err(), context.Canceled) {
		logger.Info("all clients closed successfully")
	}

	serveErr := <-serveErrCh
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}

	return errors.Join(serveErr, shutdownErr)
}

func (s *Server) Run(ctx context.Context) error {
	logger := ctxlog.Get(ctx)

	// setup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)

	// main server
	srv := &http.Server{
		Addr:        s.addr,
		Handler:     s.handler,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	if s.tlsLoader != nil {
		srv.TLSConfig = &tls.Config{
			GetCertificate: s.tlsLoader.getCertificate,
		}
	}
	group.Go(func() error {
		return s.runServer(ctx, cancel, srv)
	})

	// redirect server
	if s.httpsRedirect {
		redirectToHTTPS := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
		httpsRedirectSrv := &http.Server{
			Addr:        ":80",
			Handler:     redirectToHTTPS,
			BaseContext: func(net.Listener) context.Context { return ctx },
		}
		group.Go(func() error {
			return s.runServer(ctx, cancel, httpsRedirectSrv)
		})
	}

	if s.apiPort != 0 {
		apiSrv := &http.Server{
			Addr:        net.JoinHostPort("localhost", strconv.Itoa(s.apiPort)),
			Handler:     s.apiHandler,
			BaseContext: func(net.Listener) context.Context { return ctx },
		}
		group.Go(func() error {
			return s.runServer(ctx, cancel, apiSrv)
		})
	}

	<-ctx.Done()

	logger.Info("server is shutting down")

	return group.Wait()
}
