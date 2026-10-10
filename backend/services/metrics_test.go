package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"meerkat/config"
	"meerkat/metrics"
	"meerkat/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scrapeDefault() string {
	var sb strings.Builder
	metrics.Default.Write(&sb)
	return sb.String()
}

// series returns the value of the exact series line in the default registry (0 if absent).
func series(t *testing.T, name string) float64 {
	t.Helper()
	for _, l := range strings.Split(scrapeDefault(), "\n") {
		if v, ok := strings.CutPrefix(l, name+" "); ok {
			f, err := strconv.ParseFloat(v, 64)
			require.NoError(t, err)
			return f
		}
	}
	return 0
}

func TestJobMetricsReadJobExecutions(t *testing.T) {
	db := webhookDB(t)
	require.NoError(t, db.AutoMigrate(&models.JobExecution{}))
	reg := metrics.NewRegistry()
	RegisterJobMetrics(reg, db)

	at := time.Unix(1700000000, 0)
	require.NoError(t, db.Create(&models.JobExecution{JobName: "daily_reminders", LastRunAt: at}).Error)
	locked := at.Add(time.Hour)
	require.NoError(t, db.Create(&models.JobExecution{JobName: "calendar_sync", LastRunAt: at, LockedAt: &locked}).Error)

	var sb strings.Builder
	reg.Write(&sb)
	out := sb.String()
	assert.Contains(t, out, `meerkat_job_last_success_timestamp_seconds{job="daily_reminders"} 1.7e+09`)
	assert.Contains(t, out, `meerkat_job_last_success_timestamp_seconds{job="calendar_sync"} 1.7e+09`)
	assert.Contains(t, out, `meerkat_job_running{job="calendar_sync"} 1`)
	assert.Contains(t, out, `meerkat_job_running{job="daily_reminders"} 0`)
}

func TestWebhookDeliveryCountsResults(t *testing.T) {
	db := webhookDB(t)
	fail := `meerkat_webhook_deliveries_total{result="failure"}`
	ok := `meerkat_webhook_deliveries_total{result="success"}`
	dead := `meerkat_outbox_dead_total{kind="webhook"}`
	f0, s0, d0 := series(t, fail), series(t, ok), series(t, dead)

	msg := "boom"
	d := models.WebhookDelivery{WebhookID: 1, Attempts: 0}
	finishWebhookDelivery(db, d, 1, nil, &msg, false)
	finishWebhookDelivery(db, d, maxOutboxAttempts, nil, &msg, false)
	code := 204
	finishWebhookDelivery(db, d, 1, &code, nil, false)

	assert.Equal(t, f0+2, series(t, fail))
	assert.Equal(t, s0+1, series(t, ok))
	assert.Equal(t, d0+1, series(t, dead), "only the last failure is a dead letter")
}

func TestSendEmailCountsResults(t *testing.T) {
	fail := `meerkat_emails_total{result="failure"}`
	ok := `meerkat_emails_total{result="success"}`
	f0, s0 := series(t, fail), series(t, ok)

	// Nothing listens on port 1, so the SMTP channel fails.
	cfg := config.Config{UseSMTP: true, SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPFromEmail: "from@example.com"}
	require.Error(t, SendEmail(cfg, EmailMessage{To: "a@example.com", Subject: "s", HTML: "x"}))
	assert.Equal(t, f0+1, series(t, fail))

	// No channel configured: nothing was sent, nothing is counted.
	require.NoError(t, SendEmail(config.Config{}, EmailMessage{To: "a@example.com", Subject: "s"}))
	assert.Equal(t, f0+1, series(t, fail))
	assert.Equal(t, s0, series(t, ok))
}

func TestOutboxGauges(t *testing.T) {
	db := emailDB(t)
	reg := metrics.NewRegistry()
	RegisterOutboxMetrics(reg, db)
	require.NoError(t, db.Create(&models.WebhookDelivery{WebhookID: 1, Status: models.OutboxDead}).Error)
	require.NoError(t, db.Create(&models.WebhookDelivery{WebhookID: 1, Status: models.OutboxPending}).Error)
	require.NoError(t, db.Create(&models.EmailOutbox{ToAddr: "a", Subject: "s", HTML: "h", Status: models.OutboxPending, NextAttemptAt: time.Now()}).Error)

	var sb strings.Builder
	reg.Write(&sb)
	out := sb.String()
	assert.Contains(t, out, `meerkat_outbox_pending{kind="webhook"} 1`)
	assert.Contains(t, out, `meerkat_outbox_pending{kind="email"} 1`)
	assert.Contains(t, out, `meerkat_outbox_dead{kind="webhook"} 1`)
	assert.Contains(t, out, `meerkat_outbox_dead{kind="email"} 0`)
}
