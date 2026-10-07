package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// A document can be turned into a short two-host conversation. The AI service writes the script
// once (a model call, counted like a chat question); it is stored with the document, so replaying
// it, even on another day, costs nothing. The browser reads it aloud.
const (
	maxPodcastTurns     = 30
	maxPodcastTurnRunes = 700
	maxPodcastTitle     = 120
	minPodcastTurns     = 2
	// Longer documents are described to the hosts through their summary alone.
	podcastFullTextMax = 12_000
)

type podcastTurn struct {
	Speaker string `json:"speaker"` // "A" or "B"
	Text    string `json:"text"`
}

// podcastScript is what is stored and returned.
type podcastScript struct {
	Title    string        `json:"title"`
	Language string        `json:"language"`
	Turns    []podcastTurn `json:"turns"`
}

type podcastRequest struct {
	Language   string `json:"language"`
	Regenerate bool   `json:"regenerate"`
}

// parsePodcastRequest reads the document id and the optional body, answering the client itself
// when either is bad. The body is read through ShouldBindBodyWith, which keeps it for a second read.
func parsePodcastRequest(c *gin.Context) (id uint64, body podcastRequest, ok bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document id"})
		return 0, body, false
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body."})
			return 0, body, false
		}
	}
	body.Language = strings.TrimSpace(body.Language)
	if utf8.RuneCountInString(body.Language) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported language."})
		return 0, body, false
	}
	return id, body, true
}

// ReplayStoredPodcast answers from the stored script when there is one, before the daily allowance is
// touched: replaying costs no model call, so it must work even for someone who has used theirs up.
// Anything else (no script yet, another language, regeneration) continues to the generator.
func ReplayStoredPodcast(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, body, ok := parsePodcastRequest(c)
	if !ok {
		c.Abort()
		return
	}
	// Scoping the query to the user means other users' documents look like they don't exist.
	var count int64
	if err := initializers.DB.Model(&models.Document{}).Where("id = ? AND user_id = ?", id, user.ID).Count(&count).Error; err != nil || count == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	if !body.Regenerate {
		if stored, found := storedPodcast(uint(id)); found && stored.Language == body.Language {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"title": stored.Title, "language": stored.Language, "turns": stored.Turns, "cached": true})
			return
		}
	}
	c.Next()
}

// PodcastDocument writes the podcast script of one of the signed-in user's documents. It runs after
// ReplayStoredPodcast, so it only ever has new work to do, and it is what the allowance is charged for.
func PodcastDocument(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, body, ok := parsePodcastRequest(c)
	if !ok {
		middleware.RefundQuota(c)
		return
	}

	var document models.Document
	if err := initializers.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&document).Error; err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	if strings.TrimSpace(document.Summary) == "" {
		middleware.RefundQuota(c)
		c.JSON(http.StatusConflict, gin.H{"error": "This document has no summary to make a podcast from."})
		return
	}

	text := ""
	if document.HasContent && utf8.RuneCountInString(document.Content) <= podcastFullTextMax {
		text = document.Content
	}
	payload := gin.H{"summary": document.Summary, "text": text}
	if body.Language != "" {
		payload["language"] = body.Language
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/podcast"), bytes.NewReader(raw))
	if err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	prepareAIRequest(c, req)

	respBody, apiErr := callAIService(req)
	if apiErr != nil {
		middleware.RefundQuota(c)
		apiErr.send(c)
		return
	}
	var script podcastScript
	if err := json.Unmarshal(respBody, &script); err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadGateway, gin.H{"error": "The AI service returned an unreadable script."})
		return
	}
	script = cleanPodcast(script)
	script.Language = body.Language
	if len(script.Turns) < minPodcastTurns {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadGateway, gin.H{"error": "The AI service returned an empty script."})
		return
	}

	// Keeping the script is a convenience: if it cannot be saved the user still gets it.
	if stored, err := json.Marshal(script); err == nil {
		if err := initializers.DB.Exec("UPDATE documents SET podcast = ?::jsonb WHERE id = ?", string(stored), id).Error; err != nil {
			slog.Error("saving a podcast script failed", "document_id", id, "error", err, "request_id", middleware.RequestIDFrom(c))
		}
	}
	c.JSON(http.StatusOK, gin.H{"title": script.Title, "language": script.Language, "turns": script.Turns, "cached": false})
}

// storedPodcast reads the saved script of a document, if it has one.
func storedPodcast(documentID uint) (podcastScript, bool) {
	var raw sql.NullString
	if err := initializers.DB.Raw("SELECT podcast::text FROM documents WHERE id = ?", documentID).Scan(&raw).Error; err != nil || !raw.Valid {
		return podcastScript{}, false
	}
	var script podcastScript
	if err := json.Unmarshal([]byte(raw.String), &script); err != nil || len(script.Turns) < minPodcastTurns {
		return podcastScript{}, false
	}
	return script, true
}

// cleanPodcast bounds a script and drops anything that is not a turn by host A or B with some text.
func cleanPodcast(in podcastScript) podcastScript {
	out := podcastScript{Title: truncateRunes(strings.TrimSpace(in.Title), maxPodcastTitle), Turns: []podcastTurn{}}
	if out.Title == "" {
		out.Title = "Podcast"
	}
	for _, t := range in.Turns {
		if len(out.Turns) == maxPodcastTurns {
			break
		}
		text := truncateRunes(strings.TrimSpace(t.Text), maxPodcastTurnRunes)
		if (t.Speaker != "A" && t.Speaker != "B") || text == "" {
			continue
		}
		out.Turns = append(out.Turns, podcastTurn{Speaker: t.Speaker, Text: text})
	}
	return out
}
