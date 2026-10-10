package controllers

import (
	"os"
	"testing"

	"ai-summarizer/go-api/auth"

	"golang.org/x/crypto/bcrypt"
)

// Password hashing is made deliberately slow; the tests sign up and log in hundreds of times, so they use the fastest
// setting. The tests of the setting itself (see password_test.go) change it back.
func TestMain(m *testing.M) {
	auth.PasswordCost = bcrypt.MinCost
	os.Exit(m.Run())
}
