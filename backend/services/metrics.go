package services

import (
	"meerkat/logger"
	"meerkat/metrics"
	"meerkat/models"

	"gorm.io/gorm"
)

// RegisterJobMetrics adds gauges read from the job_executions table on every scrape: the
// last successful run of each job that keeps a row there (reminders, calendar sync,
// contact sync) and whether it holds its lock now. Alert on a timestamp that is too old.
func RegisterJobMetrics(r *metrics.Registry, db *gorm.DB) {
	load := func() []models.JobExecution {
		var jobs []models.JobExecution
		if err := db.Order("job_name").Find(&jobs).Error; err != nil {
			logger.Warn().Err(err).Msg("Could not read job executions for metrics")
		}
		return jobs
	}
	r.GaugeFunc("meerkat_job_last_success_timestamp_seconds",
		"Unix time of the last successful run of a job tracked in job_executions.", []string{"job"},
		func() []metrics.Sample {
			var out []metrics.Sample
			for _, j := range load() {
				out = append(out, metrics.Sample{Labels: []string{j.JobName}, Value: float64(j.LastRunAt.Unix())})
			}
			return out
		})
	r.GaugeFunc("meerkat_job_running",
		"1 while a job tracked in job_executions holds its lock.", []string{"job"},
		func() []metrics.Sample {
			var out []metrics.Sample
			for _, j := range load() {
				v := 0.0
				if j.LockedAt != nil {
					v = 1
				}
				out = append(out, metrics.Sample{Labels: []string{j.JobName}, Value: v})
			}
			return out
		})
}

// RegisterOutboxMetrics adds gauges read on every scrape: how many outbox entries wait for
// their (next) try and how many are dead, per kind. A growing dead count means a receiver
// or the mail server has been failing for longer than the retries cover.
func RegisterOutboxMetrics(r *metrics.Registry, db *gorm.DB) {
	count := func(table, status string) float64 {
		var n int64
		if err := db.Table(table).Where("status = ?", status).Count(&n).Error; err != nil {
			logger.Warn().Err(err).Str("table", table).Msg("Could not count outbox entries for metrics")
		}
		return float64(n)
	}
	for _, g := range []struct{ name, help, status string }{
		{"meerkat_outbox_pending", "Outbox entries waiting for their first try or a retry.", models.OutboxPending},
		{"meerkat_outbox_dead", "Outbox entries that gave up (dead letter) and are still stored.", models.OutboxDead},
	} {
		g := g
		r.GaugeFunc(g.name, g.help, []string{"kind"}, func() []metrics.Sample {
			return []metrics.Sample{
				{Labels: []string{"webhook"}, Value: count("webhook_deliveries", g.status)},
				{Labels: []string{"email"}, Value: count("email_outbox", g.status)},
			}
		})
	}
}
