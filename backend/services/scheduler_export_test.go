package services

import (
	"meerkat/logger"
	"time"
)

// Test-only helpers.

// RunAll runs every registered job once, immediately.
func (s *Scheduler) RunAll() {
	for _, j := range s.s.Jobs() {
		if err := j.RunNow(); err != nil {
			logger.Error().Err(err).Str("job", j.Name()).Msg("Failed to run job")
		}
	}
}

// NextRuns returns the next run time of every job by name.
func (s *Scheduler) NextRuns() map[string]time.Time {
	out := map[string]time.Time{}
	for _, j := range s.s.Jobs() {
		if next, err := j.NextRun(); err == nil {
			out[j.Name()] = next
		}
	}
	return out
}
