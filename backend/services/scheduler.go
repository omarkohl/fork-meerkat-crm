package services

import (
	"errors"
	"fmt"
	"meerkat/config"
	"meerkat/logger"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// schedulerStopTimeout bounds how long Stop waits for running jobs.
const schedulerStopTimeout = 5 * time.Second

// Names of the scheduled jobs.
const (
	JobReminders      = "reminders"
	JobWebhookRetries = "webhook-retries"
	JobCalendarSync   = "calendar-sync"
	JobContactSync    = "contact-sync"
	JobTrashPurge     = "trash-purge"
	JobBackup         = "backup"
)

// SchedulerTasks are the functions the scheduler runs periodically.
type SchedulerTasks struct {
	Reminders      func()
	WebhookRetries func()
	CalendarSync   func()
	ContactSync    func()
	TrashPurge     func()
	Backup         func()
}

// Scheduler runs the recurring background jobs.
type Scheduler struct {
	s gocron.Scheduler
}

// NewScheduler registers the recurring jobs: reminders daily at cfg.ReminderTime
// (in the reminder timezone), webhook retries every 5 minutes, and calendar and
// contact syncs every configured number of hours, the trash purge daily at 03:30 and, when enabled, the backup daily at cfg.BackupTime. Interval jobs also run once as
// soon as the scheduler starts. Registration failures are logged and skipped.
func NewScheduler(cfg *config.Config, t SchedulerTasks) (*Scheduler, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(cfg.GetReminderLocation()),
		// Shutdown must fit in the container stop grace period (10s) next to the HTTP server's.
		gocron.WithStopTimeout(schedulerStopTimeout))
	if err != nil {
		return nil, err
	}

	var hour, minute int
	if _, err := fmt.Sscanf(cfg.ReminderTime, "%d:%d", &hour, &minute); err != nil {
		logger.Error().Err(err).Str("reminder_time", cfg.ReminderTime).Msg("Failed to schedule job")
	} else {
		register(s, JobReminders,
			gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(hour), uint(minute), 0))), t.Reminders) //nolint:gosec // validated HH:MM
	}
	register(s, JobWebhookRetries, gocron.DurationJob(time.Minute), t.WebhookRetries, startNow)
	register(s, JobCalendarSync, gocron.DurationJob(time.Duration(cfg.CalDAVSyncIntervalHours)*time.Hour), t.CalendarSync, startNow)
	register(s, JobContactSync, gocron.DurationJob(time.Duration(cfg.CardDAVSyncIntervalHours)*time.Hour), t.ContactSync, startNow)
	register(s, JobTrashPurge, gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 30, 0))), t.TrashPurge)
	if cfg.BackupEnabled {
		var h, m int
		if _, err := fmt.Sscanf(cfg.BackupTime, "%d:%d", &h, &m); err != nil {
			logger.Error().Err(err).Str("backup_time", cfg.BackupTime).Msg("Failed to schedule job")
		} else {
			register(s, JobBackup, gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(h), uint(m), 0))), t.Backup) //nolint:gosec // validated HH:MM
		}
	}
	return &Scheduler{s: s}, nil
}

// startNow makes an interval job run once as soon as the scheduler starts.
var startNow = gocron.WithStartAt(gocron.WithStartImmediately())

func register(s gocron.Scheduler, name string, def gocron.JobDefinition, task func(), opts ...gocron.JobOption) {
	if _, err := s.NewJob(def, gocron.NewTask(task), append(opts, gocron.WithName(name))...); err != nil {
		logger.Error().Err(err).Str("job", name).Msg("Failed to schedule job")
	}
}

// Start begins running the jobs in the background.
func (s *Scheduler) Start() { s.s.Start() }

// Stop stops the scheduler and waits up to schedulerStopTimeout for running jobs.
func (s *Scheduler) Stop() {
	err := s.s.Shutdown()
	if errors.Is(err, gocron.ErrStopJobsTimedOut) {
		logger.Warn().Dur("timeout", schedulerStopTimeout).Msg("Scheduler stopped while jobs were still running")
	} else if err != nil {
		logger.Error().Err(err).Msg("Error stopping scheduler")
	}
}
