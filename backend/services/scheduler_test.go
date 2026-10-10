package services

import (
	"meerkat/config"
	"sync/atomic"
	"testing"
	"time"
)

type taskCounters struct{ reminders, webhooks, calendars, contacts, trash atomic.Int32 }

func (c *taskCounters) tasks() SchedulerTasks {
	return SchedulerTasks{
		Reminders:      func() { c.reminders.Add(1) },
		WebhookRetries: func() { c.webhooks.Add(1) },
		CalendarSync:   func() { c.calendars.Add(1) },
		ContactSync:    func() { c.contacts.Add(1) },
		TrashPurge:     func() { c.trash.Add(1) },
	}
}

func schedulerTestConfig() *config.Config {
	return &config.Config{
		ReminderTime:             "07:30",
		ReminderTimezone:         "Europe/Berlin",
		CalDAVSyncIntervalHours:  6,
		CardDAVSyncIntervalHours: 3,
	}
}

func newTestScheduler(t *testing.T, cfg *config.Config, c *taskCounters) *Scheduler {
	t.Helper()
	s, err := NewScheduler(cfg, c.tasks())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSchedulerNextRuns(t *testing.T) {
	cfg := schedulerTestConfig()
	c := &taskCounters{}
	s := newTestScheduler(t, cfg, c)
	s.Start()
	defer s.Stop()

	// Wait for the immediate first runs of the interval jobs, then let the
	// scheduler plan the following ones.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.webhooks.Load() >= 1 && c.calendars.Load() >= 1 && c.contacts.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	runs := s.NextRuns()
	now := time.Now()

	for _, name := range []string{JobReminders, JobWebhookRetries, JobCalendarSync, JobContactSync, JobTrashPurge} {
		if runs[name].IsZero() {
			t.Fatalf("job %q is not scheduled (got %v)", name, runs)
		}
	}

	// Reminders: daily at 07:30 in the configured timezone, within the next day.
	r := runs[JobReminders].In(cfg.GetReminderLocation())
	if r.Hour() != 7 || r.Minute() != 30 {
		t.Errorf("reminder next run at %s, want 07:30 Europe/Berlin", r)
	}
	// 26h allows for a DST change between now and the next run.
	if !r.After(now) || r.After(now.Add(26*time.Hour)) {
		t.Errorf("reminder next run %s not within the next day", r)
	}

	// Trash purge: daily at 03:30 in the same timezone, not at start.
	p := runs[JobTrashPurge].In(cfg.GetReminderLocation())
	if p.Hour() != 3 || p.Minute() != 30 {
		t.Errorf("trash purge next run at %s, want 03:30 Europe/Berlin", p)
	}
	if !p.After(now) || p.After(now.Add(26*time.Hour)) {
		t.Errorf("trash purge next run %s not within the next day", p)
	}

	// After their immediate first run, the others repeat every interval.
	for name, interval := range map[string]time.Duration{
		JobWebhookRetries: 5 * time.Minute,
		JobCalendarSync:   6 * time.Hour,
		JobContactSync:    3 * time.Hour,
	} {
		d := runs[name].Sub(now)
		if d < interval-time.Minute || d > interval+time.Minute {
			t.Errorf("%s next run in %s, want about %s", name, d, interval)
		}
	}
}

// RunAll runs every job now. The reminder job does not run at start, so it
// proves RunAll works; the interval jobs ran at start already, so they have run
// at least once.
func TestSchedulerRunAllRunsJobs(t *testing.T) {
	c := &taskCounters{}
	s := newTestScheduler(t, schedulerTestConfig(), c)
	s.Start()
	defer s.Stop()

	s.RunAll()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.reminders.Load() == 1 && c.trash.Load() == 1 && c.webhooks.Load() >= 1 && c.calendars.Load() >= 1 && c.contacts.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("runs: reminders=%d trash=%d webhooks=%d calendars=%d contacts=%d",
		c.reminders.Load(), c.trash.Load(), c.webhooks.Load(), c.calendars.Load(), c.contacts.Load())
}

// The interval jobs run once as soon as the scheduler starts (the services are
// rate limited, so main's explicit startup runs do not duplicate work); the daily
// reminder job waits for its time of day.
func TestSchedulerStartRunsIntervalJobsImmediately(t *testing.T) {
	c := &taskCounters{}
	s := newTestScheduler(t, schedulerTestConfig(), c)
	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.webhooks.Load() == 1 && c.calendars.Load() == 1 && c.contacts.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	if c.webhooks.Load() != 1 || c.calendars.Load() != 1 || c.contacts.Load() != 1 {
		t.Errorf("interval jobs ran webhooks=%d calendars=%d contacts=%d times at start, want 1 each",
			c.webhooks.Load(), c.calendars.Load(), c.contacts.Load())
	}
	if n := c.reminders.Load(); n != 0 {
		t.Errorf("reminder job ran %d times at start, want 0", n)
	}
	if n := c.trash.Load(); n != 0 {
		t.Errorf("trash purge job ran %d times at start, want 0", n)
	}
}

func TestSchedulerStopIsClean(t *testing.T) {
	s := newTestScheduler(t, schedulerTestConfig(), &taskCounters{})
	s.Start()
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
}
