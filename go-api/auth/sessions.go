package auth

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	// DefaultSessionTTL is how long a session lasts without use; each use extends it.
	DefaultSessionTTL = 30 * 24 * time.Hour
	// MaxSessionLifetime caps a session regardless of use, so a stolen cookie can't live forever.
	MaxSessionLifetime = 90 * 24 * time.Hour
	// touchInterval limits how often a session is written back on use.
	touchInterval = 5 * time.Minute
	maxUserAgent  = 200
)

// ErrInvalidSession covers unknown, expired and revoked sessions alike; callers must not
// distinguish them to the client.
var ErrInvalidSession = errors.New("invalid session")

// SessionTTL can be overridden in tests.
var SessionTTL = DefaultSessionTTL

// CreateSession starts a session for the user and returns the raw token to give to the client.
func CreateSession(userID uint, userAgent string) (string, *models.Session, error) {
	token, err := NewToken()
	if err != nil {
		return "", nil, err
	}
	if len(userAgent) > maxUserAgent {
		userAgent = userAgent[:maxUserAgent]
	}
	now := time.Now()
	session := &models.Session{
		UserID:     userID,
		TokenHash:  HashToken(token),
		UserAgent:  userAgent,
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.Add(SessionTTL),
	}
	if err := initializers.DB.Create(session).Error; err != nil {
		return "", nil, err
	}
	return token, session, nil
}

// LookupSession resolves a raw token to its live session and user, extending the session when
// it is used. Revoked, expired and unknown tokens all return ErrInvalidSession.
func LookupSession(token string) (*models.Session, *models.User, error) {
	if token == "" {
		return nil, nil, ErrInvalidSession
	}

	var session models.Session
	err := initializers.DB.
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", HashToken(token), time.Now()).
		First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidSession
		}
		return nil, nil, err
	}

	var user models.User
	if err := initializers.DB.First(&user, session.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidSession
		}
		return nil, nil, err
	}

	now := time.Now()
	if now.Sub(session.LastUsedAt) > touchInterval {
		expires := now.Add(SessionTTL)
		if hardLimit := session.CreatedAt.Add(MaxSessionLifetime); expires.After(hardLimit) {
			expires = hardLimit
		}
		// Best effort: failing to extend the session must not fail the request.
		initializers.DB.Model(&models.Session{}).Where("id = ?", session.ID).
			Updates(map[string]any{"last_used_at": now, "expires_at": expires})
		session.LastUsedAt, session.ExpiresAt = now, expires
	}
	return &session, &user, nil
}

// RevokeSession ends one of the user's sessions. It reports whether a live session was revoked.
func RevokeSession(userID, sessionID uint) (bool, error) {
	res := initializers.DB.Model(&models.Session{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).
		Update("revoked_at", time.Now())
	return res.RowsAffected > 0, res.Error
}

// RevokeAllSessions ends every live session of the user except keepID (0 keeps none).
func RevokeAllSessions(userID, keepID uint) error {
	return initializers.DB.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL AND id <> ?", userID, keepID).
		Update("revoked_at", time.Now()).Error
}

// ActiveSessions lists the user's live sessions, newest first.
func ActiveSessions(userID uint) ([]models.Session, error) {
	var sessions []models.Session
	err := initializers.DB.
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, time.Now()).
		Order("last_used_at DESC").Find(&sessions).Error
	return sessions, err
}

// RevokeByToken ends the session a raw token belongs to, if any. Used by logout, which must work
// even when the session is already invalid.
func RevokeByToken(token string) {
	initializers.DB.Model(&models.Session{}).
		Where("token_hash = ? AND revoked_at IS NULL", HashToken(token)).
		Update("revoked_at", time.Now())
}
