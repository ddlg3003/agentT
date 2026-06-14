// Package scheduler runs the daily digest job automatically once per day at a
// fixed local time. It is deliberately minimal: the run time is hardcoded (no
// per-PO configuration for the MVP), and the PO can always trigger an on-demand
// run through the API (POST /api/v1/jobs/daily) regardless of the schedule.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Hardcoded local time of day at which the daily job fires. The job generates
// the digest for the previous day, whose source data is finalized by then.
const (
	runHour   = 6
	runMinute = 0
)

// RunFunc generates and persists the daily digest for a date (YYYY-MM-DD).
type RunFunc func(ctx context.Context, date string) error

// Daily fires RunFunc once per day at the hardcoded time.
type Daily struct {
	run RunFunc
	log *slog.Logger
	now func() time.Time
}

// NewDaily builds the scheduler.
func NewDaily(run RunFunc, log *slog.Logger) *Daily {
	return &Daily{run: run, log: log, now: time.Now}
}

// Start launches the scheduler in a goroutine and returns immediately. The
// goroutine runs until ctx is cancelled (wired to server shutdown), so it has a
// bounded lifetime and leaks nothing.
func (d *Daily) Start(ctx context.Context) {
	go d.loop(ctx)
}

func (d *Daily) loop(ctx context.Context) {
	d.log.Info("daily scheduler started", "at", "06:00 local")
	for {
		wait := d.untilNextRun()
		timer := time.NewTimer(wait)
		d.log.Info("daily scheduler sleeping until next run", "in", wait.Round(time.Second).String())

		select {
		case <-ctx.Done():
			timer.Stop()
			d.log.Info("daily scheduler stopped")
			return
		case <-timer.C:
			// Generate the digest for yesterday (data is complete by the run time).
			date := d.now().AddDate(0, 0, -1).Format("2006-01-02")
			d.log.Info("daily scheduler firing", "date", date)
			if err := d.run(ctx, date); err != nil {
				d.log.Error("scheduled daily run failed", "date", date, "error", err)
			}
		}
	}
}

// untilNextRun returns the duration until the next occurrence of the hardcoded
// run time in local time.
func (d *Daily) untilNextRun() time.Duration {
	now := d.now()
	next := time.Date(now.Year(), now.Month(), now.Day(), runHour, runMinute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}
