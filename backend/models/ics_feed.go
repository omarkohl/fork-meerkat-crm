package models

import "time"

// IcsFeed is a user's subscribable calendar feed. Only the SHA-256 hash of the
// token is stored; the URL carrying the plaintext token is shown once. A user has
// at most one feed: regenerating replaces the hash, revoking deletes the row.
type IcsFeed struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null;uniqueIndex"`
	TokenHash string `gorm:"not null;uniqueIndex" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IcsFeedStatus is returned by the feed management endpoints.
type IcsFeedStatus struct {
	Enabled   bool       `json:"enabled"`
	CreatedAt *time.Time `json:"created_at"`
}

// IcsFeedCreateResponse is returned on creation/regeneration and includes the
// plaintext token, which is never retrievable again.
type IcsFeedCreateResponse struct {
	IcsFeedStatus
	Token string `json:"token"`
}
