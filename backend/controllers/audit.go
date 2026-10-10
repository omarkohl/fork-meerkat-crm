package controllers

import (
	"strings"

	"meerkat/models"
	"meerkat/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Audit details are machine-readable, not English: ";"-separated "key:value" segments that the
// admin page translates (frontend AuditLogPage). auditValue keeps a free-text value (a token
// or file name) from breaking that format.
func auditValue(s string) string {
	return strings.ReplaceAll(s, ";", ",")
}

func usernameOf(db *gorm.DB, userID uint) string {
	var name string
	if err := db.Model(&models.User{}).Where("id = ?", userID).Limit(1).Pluck("username", &name).Error; err != nil {
		return ""
	}
	return name
}

// auditFor records an event about the given account (nil for none), with the request's
// address and user agent. It never fails the request.
func auditFor(c *gin.Context, db *gorm.DB, userID *uint, event, detail string) {
	entry := models.AuditLog{
		UserID:    userID,
		Event:     event,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    detail,
	}
	if userID != nil {
		entry.Username = usernameOf(db, *userID)
	}
	services.WriteAudit(db, entry)
}

// audit records an event for the authenticated user.
func audit(c *gin.Context, db *gorm.DB, event, detail string) {
	var uid *uint
	if v, ok := c.Get("userID"); ok {
		if id, ok := v.(uint); ok {
			uid = &id
		}
	}
	auditFor(c, db, uid, event, detail)
}
