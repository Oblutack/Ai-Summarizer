package controllers

import (
	"ai-summarizer/go-api/mailer"
	"ai-summarizer/go-api/middleware"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// EmailDocument sends a saved summary to the signed-in user's own address. The recipient is never taken
// from the request, so this cannot be used to send mail to anyone else.
func EmailDocument(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	document, ok := ownedDocument(c, id, false)
	if !ok {
		return
	}
	if strings.TrimSpace(document.Summary) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "This document has no summary to send."})
		return
	}
	mailer.SendAsync(Mail, mailer.SummaryEmail(user.Email, document.Filename, document.Summary))
	c.JSON(http.StatusAccepted, gin.H{"message": "The summary is on its way to " + user.Email + "."})
}

type instructionsRequest struct {
	CustomInstructions string `json:"customInstructions"`
}

// SetInstructions saves the user's standing preferences for summaries ("focus on costs and deadlines").
func SetInstructions(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var body instructionsRequest
	if !bindJSON(c, &body) {
		return
	}
	instructions := strings.TrimSpace(strings.Join(strings.Fields(body.CustomInstructions), " "))
	if len([]rune(instructions)) > maxInstructionRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keep the instructions to 500 characters."})
		return
	}
	if err := updateInstructions(user.ID, instructions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save your instructions"})
		return
	}
	user.CustomInstructions = instructions
	c.JSON(http.StatusOK, gin.H{"user": userView(user)})
}
