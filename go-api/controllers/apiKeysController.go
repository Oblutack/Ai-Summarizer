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
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const maxAPIKeyNameChars = 40

func apiKeyJSON(k models.APIKey) gin.H {
	return gin.H{
		"id": k.ID, "name": k.Name, "prefix": k.Prefix,
		"createdAt": k.CreatedAt, "lastUsedAt": k.LastUsedAt, "revokedAt": k.RevokedAt,
	}
}

// CreateAPIKey makes a key for a program to use the /v1 routes. The key is in the reply once and nowhere else: only
// a hash is kept, so a lost key cannot be shown again, only replaced.
func CreateAPIKey(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &body) {
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

	key, record, err := auth.CreateAPIKey(user.ID, name)
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
	reply := apiKeyJSON(*record)
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
	out := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyJSON(k))
	}
	c.JSON(http.StatusOK, out)
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
