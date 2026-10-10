package models

import "time"

// APIKey lets a program use the /v1 routes as its owner. Only the hash of the key is stored.
type APIKey struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint
	Name       string
	Prefix     string
	KeyHash    string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	// ExpiresAt: after this moment the key no longer works (nil: it does not expire).
	ExpiresAt *time.Time
	// DailyLimit: the most requests the key may make in a day, whatever its owner's allowance (nil: only the owner's).
	DailyLimit *int
}
