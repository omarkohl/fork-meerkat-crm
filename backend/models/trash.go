package models

import "gorm.io/gorm"

// TrashContact soft-deletes a user's contact together with its notes, reminders
// and relationships, so nothing of it stays live (reminder emails, ICS feed).
// Activity links and photo files are kept so that restoring is lossless; the
// purge in the services package removes them for good. Every delete path must
// use this, inside a transaction. It lives here because the CardDAV package,
// which services imports, needs it too.
func TrashContact(tx *gorm.DB, userID, id uint) error {
	for _, model := range []any{&Reminder{}, &Note{}, &Relationship{}} {
		if err := tx.Where("contact_id = ? AND user_id = ?", id, userID).Delete(model).Error; err != nil {
			return err
		}
	}
	return tx.Where("id = ? AND user_id = ?", id, userID).Delete(&Contact{}).Error
}
