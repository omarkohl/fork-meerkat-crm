package models

import "time"

// Audit events. The list is also what the admin filter offers.
const (
	AuditLogin             = "login"
	AuditLoginFailed       = "login_failed"
	AuditLogout            = "logout"
	AuditPasswordChanged   = "password_changed"
	AuditPasswordReset     = "password_reset"
	AuditSessionRevoked    = "session_revoked"
	AuditSignOutEverywhere = "sign_out_everywhere"
	AuditExport            = "export"
	AuditAPITokenCreated   = "api_token_created"
	AuditAPITokenRevoked   = "api_token_revoked"
	AuditICSFeedCreated    = "ics_feed_created"
	AuditICSFeedRevoked    = "ics_feed_revoked"
	AuditBackupCreated     = "backup_created"
	AuditBackupDownloaded  = "backup_downloaded"
	AuditBackupDeleted     = "backup_deleted"
	AuditAdminUserUpdated  = "admin_user_updated"
	AuditAdminUserDeleted  = "admin_user_deleted"
	AuditAdminReminders    = "admin_reminders_triggered"
)

// AuditEvents lists every event, in the order the UI shows them.
var AuditEvents = []string{
	AuditLogin, AuditLoginFailed, AuditLogout, AuditPasswordChanged, AuditPasswordReset,
	AuditSessionRevoked, AuditSignOutEverywhere, AuditExport,
	AuditAPITokenCreated, AuditAPITokenRevoked, AuditICSFeedCreated, AuditICSFeedRevoked,
	AuditBackupCreated, AuditBackupDownloaded, AuditBackupDeleted,
	AuditAdminUserUpdated, AuditAdminUserDeleted, AuditAdminReminders,
}

// AuditLog is one security-relevant event. UserID is the account the event is about (the
// actor, or the account a failed login targeted; nil when there is none), Username a copy
// that outlives the account. Detail never holds secrets or what was typed in a login form.
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    *uint     `json:"user_id"`
	Username  string    `json:"username"`
	Event     string    `gorm:"not null" json:"event"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Detail    string    `json:"detail"`
}
