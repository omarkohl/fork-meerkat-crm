// Package session holds the lookups the auth middleware and the session service share. It
// is a leaf package because services imports middleware.
package session

import (
	"time"

	"meerkat/models"

	"gorm.io/gorm"
)

// TouchInterval bounds how often a session's last_seen_at is written. Reads are far more
// common than writes in SQLite, which takes a database-wide lock for every write.
const TouchInterval = time.Minute

// Find returns the user's session with this id when it is neither revoked nor expired.
// It is one read; gorm.ErrRecordNotFound means there is no such live session.
func Find(db *gorm.DB, userID uint, publicID string) (models.Session, error) {
	var s models.Session
	err := db.Where("public_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", publicID, userID, time.Now()).Take(&s).Error
	return s, err
}

// Touch stamps last_seen_at, but only when the value read by Find is older than
// TouchInterval, so most requests do not write at all.
func Touch(db *gorm.DB, s models.Session) error {
	now := time.Now()
	if now.Sub(s.LastSeenAt) < TouchInterval {
		return nil
	}
	return db.Model(&models.Session{}).Where("id = ?", s.ID).Update("last_seen_at", now).Error
}
