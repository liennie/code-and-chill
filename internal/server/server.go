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

	"golang.org/x/sync/errgroup"
)

type Server struct {
	addr            string
	handler         http.Handler
	tlsLoader       *tlsLoader
	httpsRedirect   bool
	shutdownTimeout time.Duration
}

func New(config Config, db *db.DB) *Server {
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
		handler:         newHandler(config, db),
		tlsLoader:       loader,
		httpsRedirect:   config.HTTPSRedirect,
		shutdownTimeout: config.ShutdownTimeout,
	}
}

func ptr[T any](v T) *T {
	return &v
}

func newHandler(config Config, db *db.DB) (h http.Handler) {
	fsys := os.DirFS(filepath.FromSlash(config.DataDir))

	// handlers
	mux := http.NewServeMux()

	registerHandler := func(method, path, src string, handler http.Handler) {
		slog.Info("registering handler", "method", method, "path", path, "src", src)
		mux.Handle(method+" "+path, handler)
	}

	page := templateHandler(dataFile(fsys, "templates/page.html"))

	// TODO notFoundHandler as a templated page

	registerHandler("GET", "/", "404.html", notFoundHandler(dataFile(fsys, "404.html")))

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

	// templates
	test, _ := dataFile(fsys, "md/test.md")
	registerHandler("GET", "/test", "md/test.md", page(func(ctx context.Context) (int, any) {
		return http.StatusOK, pageData{
			Title:  "Test Page",
			Year:   2026,
			Puzzle: 3,
			User: &userData{
				Name:   "Liennie",
				Avatar: "https://placedog.net/40/40",
			},
			Content: contentData{
				Parts: []partData{
					{
						MD:         string(test),
						Answer:     "123",
						WantAnswer: true,
					},
					{
						MD:         `# Part 2`,
						WantAnswer: true,
					},
				},
			},
			Puzzles: []puzzleData{
				{
					Class: puzzleClassSolvedBoth,
					Name:  "01",
				},
				{
					Class: puzzleClassSolvedOne,
					Name:  "02",
				},
				{
					Class: puzzleClassUnlocked,
					Name:  "03: Binary Diagnostic",
				},
				{
					Class:  puzzleClassLocked,
					Name:   "04: Lorem ipsum dolor sit amet",
					Unlock: ptr(time.Date(2025, 10, 29, 1, 9, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "05",
					Unlock: ptr(time.Date(2025, 10, 29, 12, 0, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "06",
					Unlock: ptr(time.Date(2025, 10, 30, 1, 9, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "07",
					Unlock: ptr(time.Date(2025, 10, 30, 12, 0, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "08",
					Unlock: ptr(time.Date(2025, 10, 31, 0, 0, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "09",
					Unlock: ptr(time.Date(2025, 10, 31, 12, 0, 0, 0, time.Local)),
				},
				{
					Class:  puzzleClassLocked,
					Name:   "10",
					Unlock: ptr(time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local)),
				},
			},
		}
	}))

	// middleware

	handler := http.Handler(mux)
	handler = newRecover(handler, internalServerErrorHandler(dataFile(fsys, "500.html")))
	handler = robotsMiddleware(handler)
	handler = hostMiddleware(config.Host, handler)
	handler = logMiddleware(handler)
	handler = sessionMiddleware(db, handler)

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
