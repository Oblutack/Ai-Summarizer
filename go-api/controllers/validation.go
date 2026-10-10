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

// validatePasswordShape checks length, the list of common passwords and obvious patterns. It can't know the user's email.
func validatePasswordShape(password string) error {
	if len(password) < minPasswordLen {
		return errors.New("Password must be at least 8 characters long.")
	}
	if len(password) > maxPasswordLen {
		return errors.New("Password must be at most 72 characters long.")
	}
	if commonPasswords[strings.ToLower(password)] || isTooSimple(password) {
		return errors.New("That password is too common. Please choose a less guessable one.")
	}
	return nil
}

// validatePassword also rejects a password that is, or contains, the user's own email name.
func validatePassword(password, email string) error {
	if err := validatePasswordShape(password); err != nil {
		return err
	}
	lower := strings.ToLower(password)
	if email != "" && (lower == strings.ToLower(email) || lower == strings.ToLower(strings.SplitN(email, "@", 2)[0])) {
		return errors.New("Your password can't be your email address.")
	}
	if containsLocalPart(password, email) {
		return errors.New("Your password can't contain your email name. Please choose a less guessable one.")
	}
	return nil
}

// normalizeLoose canonicalizes an email for lookups without rejecting malformed input.
func normalizeLoose(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
