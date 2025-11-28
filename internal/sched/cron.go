// Package sched provides scheduling utilities and a global Cron instance
// used across the application.
package sched

import (
	"github.com/robfig/cron/v3"
)

var Cron *cron.Cron

func init() {
	l := &slogLogger{}
	Cron = cron.New(
		cron.WithLogger(l),
		cron.WithChain(cron.Recover(l)),
	)
}

func Start() {
	Cron.Start()
}

func Stop() {
	<-Cron.Stop().Done()
}
