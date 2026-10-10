package models

import "time"

// RecoveryCode is a one-time code that replaces the authenticator code when the phone is
// lost. Only a hash is stored.
type RecoveryCode struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UserID    uint   `gorm:"not null;index"`
	CodeHash  string `gorm:"not null"`
	UsedAt    *time.Time
}
