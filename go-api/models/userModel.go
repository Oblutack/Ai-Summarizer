package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email    string `gorm:"unique"`
	Password string

	// EmailVerifiedAt is nil until the user proves they own the address (or signs in with Google,
	// which has already verified it).
	EmailVerifiedAt *time.Time
	// HasPassword is false for accounts created through Google sign-in, which never chose one.
	// No gorm default tag on purpose: GORM would turn a deliberate false into the default.
	HasPassword bool
	// CustomInstructions are standing preferences added to every summary prompt.
	CustomInstructions string
}

func (u User) EmailVerified() bool { return u.EmailVerifiedAt != nil }
