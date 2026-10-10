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
