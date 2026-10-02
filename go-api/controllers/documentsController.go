package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// respond runs a summary request and writes the result. When save is set, the summary and its
// source text are stored for the signed-in user; label overrides the stored title if non-empty.
// With ?stream=true the summary is streamed as server-sent events instead of returned in one piece.
func respond(c *gin.Context, build summaryBuilder, save bool, label string) {
	if wantsStream(c) {
		ar, apiErr := build(c, true)
		if apiErr != nil {
			apiErr.send(c)
			return
		}
		streamSummary(c, ar, save, label)
		return
	}

	result, apiErr := buffered(build)(c)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	if save {
		title := result.Filename
		if label != "" {
			title = label
		}
		saveDocument(c, title, result)
	}
	c.JSON(http.StatusOK, result.response())
}

func PublicSummarize(c *gin.Context)         { respond(c, buildFileRequest, false, "") }
func PublicSummarizeMultiple(c *gin.Context) { respond(c, buildFilesRequest, false, "") }
func PublicSummarizeText(c *gin.Context)     { respond(c, buildTextRequest, false, "") }
func CreateSummary(c *gin.Context)           { respond(c, buildFileRequest, true, "") }
func CreateSummaryMultiple(c *gin.Context)   { respond(c, buildFilesRequest, true, "") }
func CreateSummaryText(c *gin.Context)       { respond(c, buildTextRequest, true, "Pasted Text") }

// saveDocument stores the summary for the authenticated user. The summary is still
// returned to the client if saving fails, so a DB hiccup doesn't waste the LLM call.
func saveDocument(c *gin.Context, title string, result *aiSummary) {
	user := c.MustGet("user").(models.User)
	document := models.Document{
		Filename:   title,
		Summary:    result.Summary,
		UserID:     user.ID,
		Content:    result.Text,
		HasContent: result.Text != "",
	}
	if err := initializers.DB.Create(&document).Error; err != nil {
		_ = c.Error(err)
	}
}

func ListDocuments(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	// Content can be hundreds of KB per document and is never shown in the list.
	var documents []models.Document
	if err := initializers.DB.Omit("content").Where("user_id = ?", user.ID).Find(&documents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load documents"})
		return
	}

	c.JSON(http.StatusOK, documents)
}

func DeleteDocument(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document id"})
		return
	}

	var document models.Document
	if err := initializers.DB.Omit("content").First(&document, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	if document.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this document"})
		return
	}

	if err := initializers.DB.Delete(&document).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document"})
		return
	}
	c.Status(http.StatusOK)
}
