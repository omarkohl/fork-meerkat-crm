package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"meerkat/config"
	"meerkat/logger"
	"meerkat/metrics"
	"meerkat/models"

	"gorm.io/gorm"
)

const (
	// Sent emails are only kept to see that they went out; dead ones (mail server down for
	// all tries) stay longer so an admin can look at them. Both hold the message body.
	sentEmailRetention = 7 * 24 * time.Hour
	deadEmailRetention = 30 * 24 * time.Hour
)

var (
	// emailSender is the real send; a seam for tests.
	emailSender = SendEmail
	// startEmailDelivery runs the immediate try in the background; a seam for tests.
	startEmailDelivery = func(f func()) { go f() }
)

// QueueEmail stores msg in the email outbox and tries to send it right away in the
// background. Failures are retried with backoff by the outbox job (see ProcessEmailOutbox),
// also after a restart. Like SendEmail it does nothing when no email channel is configured
// or the recipient is empty. userID (optional) ties the row to its owner, so deleting the
// user removes it.
func QueueEmail(db *gorm.DB, cfg config.Config, userID *uint, msg EmailMessage) error {
	if msg.To == "" {
		logger.Warn().Msg("Skipping email because recipient address is empty")
		return nil
	}
	if strings.ContainsAny(msg.To, "\r\n") || strings.ContainsAny(msg.Subject, "\r\n") {
		return fmt.Errorf("refusing to queue email: line break in recipient or subject")
	}
	if !cfg.EmailEnabled() {
		logger.Warn().Str("to", msg.To).Msg("No email channel configured; email not queued")
		return nil
	}
	e := models.EmailOutbox{
		UserID: userID, ToAddr: msg.To, Subject: msg.Subject, HTML: msg.HTML,
		Status: models.OutboxPending, NextAttemptAt: time.Now(),
	}
	if err := db.Create(&e).Error; err != nil {
		return fmt.Errorf("queue email: %w", err)
	}
	startEmailDelivery(func() {
		if claimEmail(db, e.ID) {
			sendQueuedEmail(db, cfg, e)
		}
	})
	return nil
}

// claimEmail takes a pending email for this worker unless another holds a fresh claim.
func claimEmail(db *gorm.DB, id uint) bool {
	now := time.Now()
	res := db.Model(&models.EmailOutbox{}).
		Where("id = ? AND status = ? AND (locked_at IS NULL OR locked_at < ?)", id, models.OutboxPending, now.Add(-outboxLockTimeout)).
		Update("locked_at", now)
	return res.Error == nil && res.RowsAffected == 1
}

// sendQueuedEmail makes one try for a claimed row and records the outcome on it.
func sendQueuedEmail(db *gorm.DB, cfg config.Config, e models.EmailOutbox) {
	err := emailSender(cfg, EmailMessage{To: e.ToAddr, Subject: e.Subject, HTML: e.HTML})
	attempt := e.Attempts + 1
	now := time.Now()
	upd := map[string]any{"attempts": attempt, "locked_at": nil}
	switch {
	case err == nil:
		upd["status"], upd["sent_at"], upd["last_error"] = models.OutboxDelivered, now, ""
	case attempt >= maxOutboxAttempts:
		upd["status"], upd["last_error"] = models.OutboxDead, err.Error()
		metrics.OutboxDead("email")
		logger.Error().Err(err).Uint("email_id", e.ID).Int("attempts", attempt).Msg("Email gave up after the last try")
	default:
		upd["last_error"], upd["next_attempt_at"] = err.Error(), now.Add(outboxBackoff[attempt-1])
		logger.Warn().Err(err).Uint("email_id", e.ID).Int("attempt", attempt).Msg("Email failed, will retry")
	}
	if dbErr := db.Model(&models.EmailOutbox{}).Where("id = ?", e.ID).Updates(upd).Error; dbErr != nil {
		logger.Error().Err(dbErr).Uint("email_id", e.ID).Msg("Failed to save email outbox state")
	}
}

// ProcessEmailOutbox sends every pending email that is due: first tries that a restart
// interrupted and retries after a failure. Nothing happens (and no try is counted) while
// no email channel is configured.
func ProcessEmailOutbox(db *gorm.DB, cfg config.Config) {
	if !cfg.EmailEnabled() {
		return
	}
	var due []models.EmailOutbox
	now := time.Now()
	if err := db.Where("status = ? AND next_attempt_at <= ? AND (locked_at IS NULL OR locked_at < ?)",
		models.OutboxPending, now, now.Add(-outboxLockTimeout)).
		Order("next_attempt_at").Limit(100).Find(&due).Error; err != nil {
		logger.Error().Err(err).Msg("Failed to load the email outbox")
		return
	}
	var wg sync.WaitGroup
	for _, e := range due {
		if !claimEmail(db, e.ID) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sendQueuedEmail(db, cfg, e)
		}()
	}
	wg.Wait()
}

// PurgeEmailOutbox deletes sent emails after a week and dead ones after 30 days.
func PurgeEmailOutbox(db *gorm.DB, now time.Time) (int64, error) {
	res := db.Where("(status = ? AND updated_at < ?) OR (status = ? AND updated_at < ?)",
		models.OutboxDelivered, now.Add(-sentEmailRetention), models.OutboxDead, now.Add(-deadEmailRetention)).
		Delete(&models.EmailOutbox{})
	return res.RowsAffected, res.Error
}
