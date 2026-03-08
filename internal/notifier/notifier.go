// Package notifier provides functionality to send and schedule notifications for puzzle events.
package notifier

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

type Notifier struct {
	BaseURI url.URL
	Discord *DiscordNotifier

	cancel []context.CancelFunc
	wg     sync.WaitGroup
}

func New(config Config) *Notifier {
	if config.BaseURI == "" || config.Discord == nil {
		return nil
	}

	u, err := url.Parse(config.BaseURI)
	if err != nil {
		panic(fmt.Errorf("notifier: invalid base url: %w", err))
	}

	return &Notifier{
		BaseURI: *u,
		Discord: newDiscordNotifier(config.Discord),
	}
}

func (n *Notifier) Notify(ctx context.Context, pzls *puzzles.Puzzles, event puzzles.Event, puzzle puzzles.Puzzle) error {
	if n == nil {
		return fmt.Errorf("notifier is not configured")
	}

	errs := []error{}

	title := fmt.Sprintf("%s | %s :: %s", puzzle.Name, pzls.Name, event.Name)
	snippet := snippetFromMD(puzzle.Parts[0].Text)
	link := n.BaseURI.JoinPath(event.Path, "puzzle", puzzle.Path).String()

	if n.Discord != nil {
		err := n.Discord.notify(ctx,
			event.Path,
			title,
			puzzle.Name,
			snippet,
			link,
		)
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func snippetFromMD(md string) string {
	for p := range strings.SplitSeq(md, "\n\n") {
		if strings.HasPrefix(p, "#") {
			continue
		}

		return p
	}
	return ""
}

type schedule struct {
	time  time.Time
	name  string
	notif func() error
}

func (n *Notifier) Schedule(ctx context.Context, pzls *puzzles.Puzzles) {
	if n == nil {
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	now := time.Now()

	sched := []schedule{}
	for _, event := range pzls.Events {
		for _, puzzle := range event.Puzzles {
			if puzzle.Unlock.Before(now) {
				continue
			}

			sched = append(sched, schedule{
				time: puzzle.Unlock,
				name: fmt.Sprintf("%s::%s::%s", pzls.Name, event.Name, puzzle.Name),
				notif: func() error {
					return n.Notify(ctx, pzls, event, puzzle)
				},
			})
		}
	}

	slices.SortFunc(sched, func(a, b schedule) int {
		return a.time.Compare(b.time)
	})

	n.cancel = append(n.cancel, cancel)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()

		logger := ctxlog.Get(ctx)
		defer logger.Info("no more notifications")

		for i, s := range sched {
			logger.Info("next notification", "time", s.time, "puzzle", s.name, "scheduled", len(sched)-i)

			t := time.NewTimer(time.Until(s.time))

			select {
			case <-t.C:
				logger.Info("notifying", "puzzle", s.name)
				err := s.notif()
				if err != nil {
					logger.Error("notification", "error", err)
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (n *Notifier) Stop() {
	if n == nil {
		return
	}

	for _, cancel := range n.cancel {
		cancel()
	}
	clear(n.cancel)
	n.cancel = n.cancel[:0]
	n.wg.Wait()
}
