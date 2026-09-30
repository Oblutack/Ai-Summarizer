package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	maxQuestionChars   = 1_000
	maxHistoryMessages = 10
	maxHistoryChars    = 4_000
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Question string        `json:"question"`
	History  []chatMessage `json:"history"`
}

func (r *chatRequest) validate() *apiError {
	r.Question = strings.TrimSpace(r.Question)
	if r.Question == "" {
		return &apiError{http.StatusBadRequest, "Please type a question."}
	}
	if utf8.RuneCountInString(r.Question) > maxQuestionChars {
		return &apiError{http.StatusBadRequest, "Question is too long (max 1000 characters)."}
	}
	if len(r.History) > maxHistoryMessages {
		r.History = r.History[len(r.History)-maxHistoryMessages:]
	}
	for _, m := range r.History {
		if m.Role != "user" && m.Role != "assistant" {
			return &apiError{http.StatusBadRequest, "Invalid chat history."}
		}
		if utf8.RuneCountInString(m.Content) > maxHistoryChars {
			return &apiError{http.StatusBadRequest, "Chat history message is too long."}
		}
	}
	return nil
}

// ChatWithDocument answers a question about one of the signed-in user's saved documents.
func ChatWithDocument(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document id"})
		return
	}

	var body chatRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body."})
		return
	}
	if apiErr := body.validate(); apiErr != nil {
		apiErr.send(c)
		return
	}

	// Scoping the query to the user means other users' documents look like they don't exist.
	var document models.Document
	if err := initializers.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&document).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	if !document.HasContent || document.Content == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "This document was saved before chat was available. Summarize it again to chat with it."})
		return
	}

	payload, err := json.Marshal(gin.H{"text": document.Content, "question": body.Question, "history": body.History})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/chat"), bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	respBody, apiErr := callAIService(req)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	var out struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil || strings.TrimSpace(out.Answer) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "The AI service returned an empty answer."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"answer": out.Answer})
}
