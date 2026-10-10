package auth

import "golang.org/x/crypto/bcrypt"

// PasswordCost is the bcrypt work factor for new password hashes. Each step doubles the work: at 12 a hash takes a
// few tenths of a second, which nobody logging in notices and which makes guessing from a stolen database about four
// times slower than the library default of 10. Tests lower it to the minimum so they stay fast.
var PasswordCost = 12

// HashPassword hashes a password for storage, at the current PasswordCost.
func HashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
}

// NeedsRehash says whether a stored hash was made with less work than is used now. A hash can only be remade when the
// person types their password, so the login that just checked it does so (see Login).
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	return err == nil && cost < PasswordCost
}
