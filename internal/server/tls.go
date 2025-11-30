package server

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync/atomic"

	"cc/internal/sched"

	"github.com/robfig/cron/v3"
)

type tlsLoader struct {
	certFile string
	keyFile  string

	cert atomic.Pointer[tls.Certificate]

	reloadJobID *cron.EntryID
}

func newTLSLoader(certFile, keyFile string, reloadSchedule string) *tlsLoader {
	t := &tlsLoader{
		certFile: certFile,
		keyFile:  keyFile,
	}

	err := t.load()
	if err != nil {
		panic(err)
	}

	if reloadSchedule != "" {
		schedule, err := cron.ParseStandard(reloadSchedule)
		if err != nil {
			panic(fmt.Errorf("tls: reloadSchedule: %w", err))
		}
		reloadJobID := sched.Cron.Schedule(schedule, cron.FuncJob(t.reload))
		t.reloadJobID = &reloadJobID
	}

	return t
}

func (l *tlsLoader) Close() error {
	if l.reloadJobID != nil {
		sched.Cron.Remove(*l.reloadJobID)
		l.reloadJobID = nil
	}
	return nil
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

func (l *tlsLoader) reload() {
	err := l.load()
	if err != nil {
		slog.Error("tls reload failed", "error", err)
	} else {
		slog.Info("tls cert reloaded")
	}
}
