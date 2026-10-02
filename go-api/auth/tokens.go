// Package auth holds the pieces of authentication that don't belong to a single handler:
// token generation, server-side sessions, session cookies and single-use email tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns a random, unguessable token (256 bits) safe to put in a cookie or URL.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is what gets stored. The tokens are already high-entropy random values, so a plain
// SHA-256 is the right tool (a slow password hash would only add latency to every request).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
