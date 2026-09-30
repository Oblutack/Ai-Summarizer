package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func PublicSummarize(c *gin.Context) {
	result, apiErr := summarizeFile(c)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

func PublicSummarizeText(c *gin.Context) {
	result, apiErr := summarizeText(c)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

func CreateSummary(c *gin.Context) {
	result, apiErr := summarizeFile(c)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	saveDocument(c, result.Filename, result.Summary)
	c.JSON(http.StatusOK, result)
}

func CreateSummaryText(c *gin.Context) {
	result, apiErr := summarizeText(c)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	saveDocument(c, "Pasted Text", result.Summary)
	c.JSON(http.StatusOK, result)
}

// saveDocument stores the summary for the authenticated user. The summary is still
// returned to the client if saving fails, so a DB hiccup doesn't waste the LLM call.
func saveDocument(c *gin.Context, filename, summary string) {
	user := c.MustGet("user").(models.User)
	document := models.Document{Filename: filename, Summary: summary, UserID: user.ID}
	if err := initializers.DB.Create(&document).Error; err != nil {
		_ = c.Error(err)
	}
}

func ListDocuments(c *gin.Context) {
	user := c.MustGet("user").(models.User)

	var documents []models.Document
	if err := initializers.DB.Where("user_id = ?", user.ID).Find(&documents).Error; err != nil {
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
	if err := initializers.DB.First(&document, id).Error; err != nil {
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
