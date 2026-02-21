package db

import (
	"github.com/robfig/cron/v3"
)

type backupJob struct {
	*DB
}

var _ cron.Job = (*backupJob)(nil)

func (j *backupJob) Run() {
	j.Backup()
}
