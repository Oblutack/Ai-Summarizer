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
}
