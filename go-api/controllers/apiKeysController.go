package controllers

import (
	"ai-summarizer/go-api/auth"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const maxAPIKeyNameChars = 40

const (
	maxAPIKeyExpiryDays = 366
	maxAPIKeyDailyLimit = 1_000_000
)

func apiKeyJSON(k models.APIKey, use auth.KeyUse) gin.H {
	return gin.H{
		"id": k.ID, "name": k.Name, "prefix": k.Prefix,
		"createdAt": k.CreatedAt, "lastUsedAt": k.LastUsedAt, "revokedAt": k.RevokedAt,
		"expiresAt": k.ExpiresAt, "dailyLimit": k.DailyLimit,
		"usedToday": use.Today, "usedTotal": use.Total,
	}
}

// apiKeysJSON describes keys with what each has done.
func apiKeysJSON(keys []models.APIKey) []gin.H {
	ids := make([]uint, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.ID)
	}
	use, _ := auth.APIKeyUse(ids)
	out := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyJSON(k, use[k.ID]))
	}
	return out
}

// CreateAPIKey makes a key for a program to use the /v1 routes. The key is in the reply once and nowhere else: only
// a hash is kept, so a lost key cannot be shown again, only replaced.
func CreateAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		Name string `json:"name"`
		// ExpiresInDays: the key stops working after this many days (absent or 0: it never expires).
		ExpiresInDays *int `json:"expiresInDays"`
		// DailyLimit: the most requests the key may make in a day (absent: only its owner's allowance limits it).
		DailyLimit *int `json:"dailyLimit"`
	}
	if !bindJSON(c, &body) {
		return
	}
	var expires *time.Time
	if d := body.ExpiresInDays; d != nil && *d != 0 {
		if *d < 0 || *d > maxAPIKeyExpiryDays {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("A key can expire in 1 to %d days.", maxAPIKeyExpiryDays)})
			return
		}
		at := time.Now().Add(time.Duration(*d) * 24 * time.Hour)
		expires = &at
	}
	if l := body.DailyLimit; l != nil && (*l < 1 || *l > maxAPIKeyDailyLimit) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("A key's daily limit can be 1 to %d requests.", maxAPIKeyDailyLimit)})
		return
	}
	name := strings.Join(strings.Fields(body.Name), " ")
	if name == "" {
		name = "API key"
	}
	if utf8.RuneCountInString(name) > maxAPIKeyNameChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Give the key a name of at most %d characters.", maxAPIKeyNameChars)})
		return
	}

	key, record, err := auth.CreateAPIKey(user.ID, auth.APIKeyOptions{Name: name, ExpiresAt: expires, DailyLimit: body.DailyLimit})
	if err != nil {
		if errors.Is(err, auth.ErrTooManyAPIKeys) {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("You already have %d API keys. Revoke one to make another.", auth.MaxAPIKeysPerUser),
				"code":  "too_many_api_keys",
			})
			return
		}
		slog.Error("creating an api key failed", "user_id", user.ID, "error", err, "request_id", middleware.RequestIDFrom(c))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create the API key."})
		return
	}
	securityEvent(c, "api_key_created", user.ID)
	reply := apiKeyJSON(*record, auth.KeyUse{})
	reply["key"] = key
	c.JSON(http.StatusCreated, reply)
}

// ListAPIKeys shows the person's keys: never the keys themselves, only what tells them apart.
func ListAPIKeys(c *gin.Context) {
	user := middleware.CurrentUser(c)
	keys, err := auth.APIKeysOf(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load your API keys."})
		return
	}
	c.JSON(http.StatusOK, apiKeysJSON(keys))
}

// RevokeAPIKey ends a key at once.
func RevokeAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key id."})
		return
	}
	revoked, err := auth.RevokeAPIKey(user.ID, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke the API key."})
		return
	}
	if !revoked {
		c.JSON(http.StatusNotFound, gin.H{"error": "That API key was not found."})
		return
	}
	securityEvent(c, "api_key_revoked", user.ID)
	c.JSON(http.StatusOK, gin.H{"message": "The API key was revoked."})
}
