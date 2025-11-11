// Package server provides HTTP server setup and lifecycle management for the application.
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
	"strings"
	"time"

	"cc/internal/auth"
	"cc/internal/ctxlog"
	"cc/internal/db"
	"cc/internal/puzzles"
	"cc/internal/session"

	"golang.org/x/sync/errgroup"
)

type Server struct {
	addr            string
	handler         http.Handler
	tlsLoader       *tlsLoader
	httpsRedirect   bool
	shutdownTimeout time.Duration
}

func New(config Config, db *db.DB, session *session.Store, auth *auth.Auth, puzzles *puzzles.Puzzles) *Server {
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
		if config.TLSReloadInterval <= 0 {
			config.TLSReloadInterval = 24 * time.Hour
		}

		loader = newTLSLoader(config.TLSCertFile, config.TLSKeyFile, config.TLSReloadInterval)
	} else if config.TLSCertFile != "" || config.TLSKeyFile != "" {
		panic("server: both tlsCertFile and tlsKeyFile must be set to enable TLS")
	}

	return &Server{
		addr:            fmt.Sprintf("%s:%d", config.Host, config.Port),
		handler:         newHandler(config, db, session, auth, puzzles),
		tlsLoader:       loader,
		httpsRedirect:   config.HTTPSRedirect,
		shutdownTimeout: config.ShutdownTimeout,
	}
}

func handlerRegistrar(prefix string, mux *http.ServeMux) func(method, path, src string, handler http.Handler) {
	return func(method, path, src string, handler http.Handler) {
		slog.Info("registering handler", "method", method, "path", prefix+path, "src", src)
		mux.Handle(method+" "+path, handler)
	}
}

func newHandler(config Config, db *db.DB, session *session.Store, auth *auth.Auth, pzls *puzzles.Puzzles) (h http.Handler) {
	fsys := os.DirFS(filepath.FromSlash(config.DataDir))

	// handlers
	mux := http.NewServeMux()
	reg := handlerRegistrar("", mux)

	page := templateHandler(dataFile(fsys, "templates/page.html"))
	unauthorizedHandler := page(mdDataFunc(http.StatusUnauthorized, "401: Unauthorized", readFile(fsys, "md/401.md")))
	notFoundHandler := page(mdDataFunc(http.StatusNotFound, "404: Not Found", readFile(fsys, "md/404.md")))
	internalErrorHandler := page(mdDataFunc(http.StatusInternalServerError, "500: Internal Server Error", readFile(fsys, "md/500.md")))

	eventMiddleware := func(event puzzles.Event) func(http.Handler) http.Handler {
		return func(handler http.Handler) http.Handler {
			handler = puzzlesMiddleware(event, handler)
			handler = darkModeMiddleware(handler)
			handler = userMiddleware(auth, handler)
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
	reg("GET", "/", "md/404.md", rootMiddleware(notFoundHandler))
	reg("GET", "/{$}", "redirect", http.RedirectHandler("/"+e, http.StatusTemporaryRedirect))
	reg("GET", "/events", "redirect", http.RedirectHandler("/"+e+"/events", http.StatusTemporaryRedirect))
	reg("GET", "/rules", "redirect", http.RedirectHandler("/"+e+"/rules", http.StatusTemporaryRedirect))
	reg("GET", "/leaderboard", "redirect", http.RedirectHandler("/"+e+"/leaderboard", http.StatusTemporaryRedirect))
	reg("GET", "/contact", "redirect", http.RedirectHandler("/"+e+"/contact", http.StatusTemporaryRedirect))
	reg("GET", "/latest", "redirect", http.RedirectHandler("/"+e+"/latest", http.StatusTemporaryRedirect))

	reg("GET", "/login", "redirect", http.RedirectHandler("/"+e+"/login", http.StatusTemporaryRedirect))
	reg("GET", "/login/fail", "redirect", http.RedirectHandler("/"+e+"/login/fail", http.StatusTemporaryRedirect))
	reg("GET", "/profile", "redirect", http.RedirectHandler("/"+e+"/profile", http.StatusTemporaryRedirect))
	reg("GET", "/logout", "redirect", http.RedirectHandler("/"+e+"/logout", http.StatusTemporaryRedirect))

	reg("GET", "/auth/discord", "discordAuthCallback", rootMiddleware(userMux(
		http.RedirectHandler("/", http.StatusSeeOther),
		discordAuthCallback(auth)),
	))

	// events
	lockedDataFunc := mdDataFunc(http.StatusNotFound, "Puzzle locked", readFile(fsys, "md/locked.md"))
	for _, event := range pzls.Events {
		e = event.Path

		evMux := http.NewServeMux()
		middleware := eventMiddleware(event)
		reg("GET", "/"+e+"/", "mux", http.StripPrefix("/"+e, middleware(evMux)))
		reg("GET", "/"+e, "md/home.md", http.StripPrefix("/"+e, middleware(page(mdDataFunc(http.StatusOK, "", readFile(fsys, "md/home.md"))))))

		reg := handlerRegistrar("/"+e, evMux)

		reg("GET", "/", "md/404.md", notFoundHandler)
		reg("GET", "/events", "md/events.md", eventsMiddleware(pzls.Events, page(mdDataFunc(http.StatusOK, "Events", readFile(fsys, "md/events.md")))))
		reg("GET", "/rules", "md/rules.md", page(mdDataFunc(http.StatusOK, "Rules", readFile(fsys, "md/rules.md"))))
		// reg("GET", "/leaderboard", "", nil)
		reg("GET", "/contact", "md/contact.md", page(mdDataFunc(http.StatusOK, "Contact", readFile(fsys, "md/contact.md"))))
		reg("GET", "/latest", "latestPuzzleRedirect", latestPuzzleRedirect(event))

		reg("GET", "/login", "md/login.md", userMux(
			http.RedirectHandler("/"+e+"/profile", http.StatusSeeOther),
			page(mdDataFunc(http.StatusOK, "Log In", readFile(fsys, "md/login.md")))),
		)
		reg("GET", "/login/discord", "discordAuthHandler", userMux(
			http.RedirectHandler("/"+e+"/profile", http.StatusSeeOther),
			discordAuthRedirect(auth, event),
		))
		reg("GET", "/login/fail", "md/401.fail.md", userMux(
			http.RedirectHandler("/"+e+"/profile", http.StatusSeeOther),
			page(mdDataFunc(http.StatusUnauthorized, "401: Unauthorized", readFile(fsys, "md/401.fail.md"))),
		))
		reg("GET", "/profile", "md/profile.md", userMux(
			eventsMiddleware(pzls.Events, page(mdDataFunc(http.StatusOK, "Profile", readFile(fsys, "md/profile.md")))),
			http.RedirectHandler("/"+e+"/login", http.StatusSeeOther),
		))
		reg("GET", "/logout", "logoutHandler", userMux(
			logoutHandler(event),
			http.RedirectHandler("/"+e, http.StatusSeeOther),
		))

		for _, puzzle := range event.Puzzles {
			p := puzzle.Path

			reg("GET", "/puzzle/"+p, "puzzleDataFunc", page(puzzleDataFunc(puzzle, lockedDataFunc)))
			reg("GET", "/puzzle/"+p+"/input", "puzzleInputHandler", userMux(
				puzzleInputHandler(puzzle, page(lockedDataFunc)),
				unauthorizedHandler,
			))
			reg("GET", "/puzzle/"+p+"/answer", "redirect", http.RedirectHandler("/"+e+"/puzzle/"+p, http.StatusTemporaryRedirect))
			reg("POST", "/puzzle/"+p+"/answer", "puzzleAnswerHandler", userMux(
				puzzleAnswerHandler(),
				unauthorizedHandler,
			))
		}
	}

	// global middleware
	handler := http.Handler(mux)
	handler = robotsMiddleware(handler)
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
		go s.tlsLoader.reloadLoop(ctx)
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

	<-ctx.Done()

	logger.Info("server is shutting down")

	return group.Wait()
}
