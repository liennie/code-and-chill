package session

import (
	"log/slog"
	"time"

	"cc/internal/db"

	"github.com/robfig/cron/v3"
)

type cleanupJob struct {
	*Store
}

var _ cron.Job = (*cleanupJob)(nil)

func (j *cleanupJob) Run() {
	slog.Info("cleaning up sessions")

	now := time.Now()

	count := 0
	err := j.db.Update(func(tx *db.Tx) error {
		to := expireKey(now)

		sessionBucket := j.bucketSession.Open(tx)
		expireBucket := j.bucketSessionExpire.Open(tx)
		for _, sessions := range expireBucket.Range("", to) {
			for _, id := range sessions.IDs {
				err := sessionBucket.Delete(id)
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
		slog.Error("session cleanup failed", "error", err, "duration", dur.String())
	} else {
		slog.Info("cleaned up sessions", "count", count, "duration", dur.String())
	}
}
