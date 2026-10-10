package auth

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// APIKeyPrefix starts every key, so a key is easy to recognize (and for secret scanners to find) when it leaks.
	APIKeyPrefix = "ink_"
	// APIKeyPrefixLength is how much of a key is kept in the open to tell keys apart: "ink_" and four more characters.
	APIKeyPrefixLength = len(APIKeyPrefix) + 4
	// MaxAPIKeysPerUser is how many live keys one person may hold.
	MaxAPIKeysPerUser = 5
	// apiKeyTouchInterval limits how often "last used" is written back.
	apiKeyTouchInterval = 5 * time.Minute
)

var (
	// ErrInvalidAPIKey covers unknown and revoked keys alike; callers must not tell the client which.
	ErrInvalidAPIKey = errors.New("invalid api key")
	// ErrTooManyAPIKeys is returned when the person already holds MaxAPIKeysPerUser live keys.
	ErrTooManyAPIKeys = errors.New("too many api keys")
)

// LooksLikeAPIKey says whether a bearer token is in the shape of an API key, so other tokens are not looked up as one.
func LooksLikeAPIKey(token string) bool {
	return strings.HasPrefix(token, APIKeyPrefix) && len(token) > APIKeyPrefixLength
}

// CreateAPIKey makes a key for the user and returns it in the clear, once: only its hash is kept.
func CreateAPIKey(userID uint, name string) (string, *models.APIKey, error) {
	token, err := NewToken()
	if err != nil {
		return "", nil, err
	}
	key := APIKeyPrefix + token
	record := &models.APIKey{UserID: userID, Name: name, Prefix: key[:APIKeyPrefixLength], KeyHash: HashToken(key)}

	err = initializers.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the owner's row so two requests arriving together cannot both squeeze under the limit.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&models.User{}, userID).Error; err != nil {
			return err
		}
		var live int64
		if err := tx.Model(&models.APIKey{}).Where("user_id = ? AND revoked_at IS NULL", userID).Count(&live).Error; err != nil {
			return err
		}
		if live >= MaxAPIKeysPerUser {
			return ErrTooManyAPIKeys
		}
		return tx.Create(record).Error
	})
	if err != nil {
		return "", nil, err
	}
	return key, record, nil
}

// LookupAPIKey resolves a key to its record and owner. Unknown and revoked keys return ErrInvalidAPIKey.
func LookupAPIKey(key string) (*models.APIKey, *models.User, error) {
	if !LooksLikeAPIKey(key) {
		return nil, nil, ErrInvalidAPIKey
	}
	var record models.APIKey
	err := initializers.DB.Where("key_hash = ? AND revoked_at IS NULL", HashToken(key)).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidAPIKey
		}
		return nil, nil, err
	}
	var user models.User
	if err := initializers.DB.First(&user, record.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidAPIKey
		}
		return nil, nil, err
	}
	now := time.Now()
	if record.LastUsedAt == nil || now.Sub(*record.LastUsedAt) > apiKeyTouchInterval {
		// Best effort: failing to note the use must not fail the request.
		initializers.DB.Model(&models.APIKey{}).Where("id = ?", record.ID).Update("last_used_at", now)
		record.LastUsedAt = &now
	}
	return &record, &user, nil
}

// APIKeysOf lists the user's keys, newest first, live and revoked.
func APIKeysOf(userID uint) ([]models.APIKey, error) {
	var keys []models.APIKey
	err := initializers.DB.Where("user_id = ?", userID).Order("id DESC").Find(&keys).Error
	return keys, err
}

// RevokeAPIKey ends one of the user's keys. It reports whether a live key was revoked.
func RevokeAPIKey(userID, keyID uint) (bool, error) {
	res := initializers.DB.Model(&models.APIKey{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", keyID, userID).
		Update("revoked_at", time.Now())
	return res.RowsAffected > 0, res.Error
}
