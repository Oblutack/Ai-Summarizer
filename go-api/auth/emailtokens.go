package auth

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"errors"
	"time"
)

// Lifetimes of emailed tokens. Reset links are short-lived because they grant account access.
const (
	VerifyEmailTTL   = 24 * time.Hour
	ResetPasswordTTL = time.Hour
)

// ErrInvalidToken covers unknown, expired, already-used and wrong-purpose tokens alike.
var ErrInvalidToken = errors.New("invalid or expired token")

// IssueEmailToken creates a fresh single-use token for the user and purpose, invalidating any
// earlier unused ones, and returns the raw token to put in the emailed link.
func IssueEmailToken(userID uint, purpose string, ttl time.Duration) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	tx := initializers.DB.Begin()
	if err := tx.Model(&models.EmailToken{}).
		Where("user_id = ? AND purpose = ? AND used_at IS NULL", userID, purpose).
		Update("used_at", now).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	row := models.EmailToken{UserID: userID, Purpose: purpose, TokenHash: HashToken(token), CreatedAt: now, ExpiresAt: now.Add(ttl)}
	if err := tx.Create(&row).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	return token, tx.Commit().Error
}

// ConsumeEmailToken validates a token and marks it used, returning the user it belongs to. The
// update is conditional on the token still being unused, so it can be redeemed only once even
// under concurrent requests.
func ConsumeEmailToken(token, purpose string) (uint, error) {
	if token == "" {
		return 0, ErrInvalidToken
	}
	var row models.EmailToken
	res := initializers.DB.Raw(
		`UPDATE email_tokens SET used_at = ?
		 WHERE token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?
		 RETURNING id, user_id`,
		time.Now(), HashToken(token), purpose, time.Now(),
	).Scan(&row)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, ErrInvalidToken
	}
	return row.UserID, nil
}
