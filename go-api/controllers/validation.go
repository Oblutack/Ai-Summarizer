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

// commonPasswords are rejected outright: they are the first thing any guessing attack tries.
// (Lowercased; length rules are applied separately.)
var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password12": true, "password123": true, "passw0rd": true,
	"12345678": true, "123456789": true, "1234567890": true, "11111111": true, "00000000": true,
	"qwertyui": true, "qwerty123": true, "qwertyuiop": true, "qwerty12": true, "1q2w3e4r": true,
	"iloveyou": true, "letmein1": true, "welcome1": true, "welcome123": true, "admin123": true,
	"abc12345": true, "abcd1234": true, "monkey123": true, "dragon123": true, "football1": true,
	"baseball1": true, "sunshine1": true, "princess1": true, "superman1": true, "trustno1": true,
	"changeme": true, "changeme1": true, "p@ssw0rd": true, "p@ssword": true, "master123": true,
}

// validatePasswordShape checks length and the blocklist. It can't know the user's email.
func validatePasswordShape(password string) error {
	if len(password) < minPasswordLen {
		return errors.New("Password must be at least 8 characters long.")
	}
	if len(password) > maxPasswordLen {
		return errors.New("Password must be at most 72 characters long.")
	}
	if commonPasswords[strings.ToLower(password)] {
		return errors.New("That password is too common. Please choose a less guessable one.")
	}
	return nil
}

// validatePassword also rejects a password that is just the user's own email address.
func validatePassword(password, email string) error {
	if err := validatePasswordShape(password); err != nil {
		return err
	}
	lower := strings.ToLower(password)
	if email != "" && (lower == strings.ToLower(email) || lower == strings.ToLower(strings.SplitN(email, "@", 2)[0])) {
		return errors.New("Your password can't be your email address.")
	}
	return nil
}

// normalizeLoose canonicalizes an email for lookups without rejecting malformed input.
func normalizeLoose(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
