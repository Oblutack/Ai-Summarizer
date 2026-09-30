package controllers

import (
	"errors"
	"net/mail"
	"strings"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores (or rejects) anything beyond 72 bytes
	maxEmailLen    = 254
)

// normalizeEmail trims and lowercases an address and checks that it is a bare, valid email.
func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > maxEmailLen {
		return "", errors.New("Please enter a valid email address.")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", errors.New("Please enter a valid email address.")
	}
	return email, nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return errors.New("Password must be at least 8 characters long.")
	}
	if len(password) > maxPasswordLen {
		return errors.New("Password must be at most 72 characters long.")
	}
	return nil
}

// normalizeLoose canonicalizes an email for lookups without rejecting malformed input.
func normalizeLoose(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
