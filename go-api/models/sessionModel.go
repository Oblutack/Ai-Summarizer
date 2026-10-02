package models

import "time"

// Session is a server-side login. Only the hash of its token is stored.
type Session struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint
	TokenHash  string
	UserAgent  string
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

// Purposes of an EmailToken.
const (
	TokenVerifyEmail   = "verify_email"
	TokenResetPassword = "reset_password"
)

// EmailToken is a single-use token sent in an emailed link. Only its hash is stored.
type EmailToken struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	Purpose   string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}
