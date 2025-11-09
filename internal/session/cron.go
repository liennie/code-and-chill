package session

import (
	"cc/internal/db"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

type slogLogger struct{}

var _ cron.Logger = (*slogLogger)(nil)

func (l *slogLogger) Info(msg string, keysAndValues ...any) {
	slog.Info("cron: "+msg, keysAndValues...)
}

func (l *slogLogger) Error(err error, msg string, keysAndValues ...any) {
	slog.Error("cron: "+msg, append(keysAndValues, []any{"error", err})...)
}

type cleanupJob struct {
	*Store
}

var _ cron.Job = (*cleanupJob)(nil)

func (j *cleanupJob) Run() {
	now := time.Now()

	count := 0
	err := j.db.Update(func(tx *db.Tx) error {
		to := expireKey(now)

		sessionBucket := tx.Session()
		dataBucket := db.SessionData[Data](tx)
		expireBucket := tx.SessionExpire()
		for _, sessions := range expireBucket.Range("", to) {
			for _, id := range sessions.IDs {
				err := sessionBucket.Delete(id)
				if err != nil {
					return err
				}

				err = dataBucket.Delete(id)
				if err != nil {
					return err
				}

				count++
			}
		}
		return expireBucket.DeleteRange("", to)
	})

	dur := time.Since(now)

	if err != nil {
		slog.Error("cleanup failed", "error", err, "duration", dur.String())
	} else {
		slog.Info("cleanup", "count", count, "duration", dur.String())
	}
}
