package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"
	"woc/internal/ctxlog"
)

type tlsLoader struct {
	certFile string
	keyFile  string
	interval time.Duration

	cert atomic.Pointer[tls.Certificate]
}

func newTLSLoader(certFile, keyFile string, interval time.Duration) *tlsLoader {
	t := &tlsLoader{
		certFile: certFile,
		keyFile:  keyFile,
		interval: interval,
	}

	err := t.load()
	if err != nil {
		panic(err)
	}

	return t
}

func (l *tlsLoader) load() error {
	c, err := tls.LoadX509KeyPair(l.certFile, l.keyFile)
	if err != nil {
		return fmt.Errorf("load tls cert: %w", err)
	}

	l.cert.Store(&c)
	return nil
}

func (l *tlsLoader) getCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return l.cert.Load(), nil
}

func (l *tlsLoader) reloadLoop(ctx context.Context) {
	logger := ctxlog.Get(ctx)

	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			err := l.load()
			if err != nil {
				logger.Error("reload tls cert", "error", err)
			} else {
				logger.Info("reloaded tls cert")
			}
		}
	}
}
