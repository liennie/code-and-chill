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
	"woc/internal/ctxlog"
	"woc/internal/db"
	"woc/internal/puzzles"

	"golang.org/x/sync/errgroup"
)

type Server struct {
	addr            string
	handler         http.Handler
	tlsLoader       *tlsLoader
	httpsRedirect   bool
	shutdownTimeout time.Duration
}

func New(config Config, puzzles *puzzles.Puzzles, db *db.DB) *Server {
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
		handler:         newHandler(config, puzzles, db),
		tlsLoader:       loader,
		httpsRedirect:   config.HTTPSRedirect,
		shutdownTimeout: config.ShutdownTimeout,
	}
}

func newHandler(config Config, pzls *puzzles.Puzzles, db *db.DB) (h http.Handler) {
	fsys := os.DirFS(filepath.FromSlash(config.DataDir))

	// handlers
	mux := http.NewServeMux()

	registerHandler := func(method, path, src string, handler http.Handler) {
		slog.Info("registering handler", "method", method, "path", path, "src", src)
		mux.Handle(method+" "+path, handler)
	}

	page := templateHandler(dataFile(fsys, "templates/page.html"))

	notFoundHandler := func(event puzzles.Event) http.Handler {
		return puzzlesMiddleware(
			event, page(mdDataFunc(http.StatusNotFound, "404: Not Found", readFile(fsys, "md/404.md"))),
		)
	}

	internalErrorHandler := func(event puzzles.Event) http.Handler {
		return puzzlesMiddleware(
			event, page(mdDataFunc(http.StatusInternalServerError, "500: Internal Server Error", readFile(fsys, "md/500.md"))),
		)
	}

	registerHandler("GET", "/", "md/404.md", notFoundHandler(pzls.Default))

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

		registerHandler("GET", subPath, p, cachedHandler(dataFile(fsys, p)))
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("server: walk www directory: %w", err))
	}

	// redirects
	registerHandler("GET", "/{$}", "redirect", http.RedirectHandler(fmt.Sprintf("/%s", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/events", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/events", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/rules", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/rules", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/leaderboard", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/leaderboard", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/contact", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/contact", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/login", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/login", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/profile", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/profile", pzls.Default.Path), http.StatusTemporaryRedirect))
	registerHandler("GET", "/latest", "redirect", http.RedirectHandler(fmt.Sprintf("/%s/latest", pzls.Default.Path), http.StatusTemporaryRedirect))

	// puzzles
	lockedDataFunc := mdDataFunc(http.StatusNotFound, "Puzzle locked", readFile(fsys, "md/locked.md"))
	for _, event := range pzls.Events {
		wrap := func(handler http.Handler) http.Handler {
			handler = newRecover(handler, internalErrorHandler(event))
			handler = puzzlesMiddleware(event, handler)
			return handler
		}

		registerHandler("GET", fmt.Sprintf("/%s/", event.Path), "md/404.md", notFoundHandler(event))
		registerHandler("GET", fmt.Sprintf("/%s", event.Path), "md/home.md", wrap(page(mdDataFunc(http.StatusOK, "", readFile(fsys, "md/home.md")))))
		registerHandler("GET", fmt.Sprintf("/%s/events", event.Path), "md/events.md", eventsMiddleware(pzls.Events, wrap(page(mdDataFunc(http.StatusOK, "Events", readFile(fsys, "md/events.md"))))))
		registerHandler("GET", fmt.Sprintf("/%s/rules", event.Path), "md/rules.md", wrap(page(mdDataFunc(http.StatusOK, "Rules", readFile(fsys, "md/rules.md")))))
		// registerHandler("GET", fmt.Sprintf("/%s/leaderboard", event), "", nil)
		registerHandler("GET", fmt.Sprintf("/%s/contact", event.Path), "md/contact.md", wrap(page(mdDataFunc(http.StatusOK, "Contact", readFile(fsys, "md/contact.md")))))
		// registerHandler("GET", fmt.Sprintf("/%s/login", event), "", nil)
		// registerHandler("GET", fmt.Sprintf("/%s/profile", event), "", nil)
		registerHandler("GET", fmt.Sprintf("/%s/latest", event.Path), "redirect", latestPuzzleRedirect(event))

		for i, puzzle := range event.Puzzles {
			registerHandler("GET", fmt.Sprintf("/%s/puzzle/%d", event.Path, i+1), fmt.Sprintf("puzzleDataFunc(%s/%d)", event.Path, i+1), wrap(page(puzzleDataFunc(i+1, puzzle, lockedDataFunc))))
			registerHandler("GET", fmt.Sprintf("/%s/puzzle/%d/input", event.Path, i+1), fmt.Sprintf("puzzleInputHandler(%s/%d)", event.Path, i+1), wrap(puzzleInputHandler(i+1, puzzle, page(lockedDataFunc))))
			registerHandler("GET", fmt.Sprintf("/%s/puzzle/%d/answer", event.Path, i+1), "redirect", http.RedirectHandler(fmt.Sprintf("/%s/puzzle/%d", event.Path, i+1), http.StatusTemporaryRedirect))
			registerHandler("POST", fmt.Sprintf("/%s/puzzle/%d/answer", event.Path, i+1), fmt.Sprintf("puzzleAnswerHandler(%s/%d)", event.Path, i+1), puzzleAnswerHandler())
		}
	}

	// middleware
	handler := http.Handler(mux)
	handler = darkModeMiddleware(handler)
	handler = sessionMiddleware(db, handler)
	handler = robotsMiddleware(handler)
	handler = hostMiddleware(config.Host, handler)
	handler = newRecover(handler, internalErrorHandler(pzls.Default))
	handler = logMiddleware(handler)
	handler = pageDataBaseMiddleware(pzls.Name, handler)

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
