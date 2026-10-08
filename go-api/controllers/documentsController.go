package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// respond runs a summary request and writes the result. When save is set, the summary and its
// source text are stored for the signed-in user; label overrides the stored title if non-empty.
// With ?stream=true the summary is streamed as server-sent events instead of returned in one piece.
func respond(c *gin.Context, build summaryBuilder, save bool, label string) {
	if wantsStream(c) {
		ar, apiErr := build(c, true)
		if apiErr != nil {
			middleware.RefundQuota(c) // nothing was summarized, so nothing should be charged
			apiErr.send(c)
			return
		}
		streamSummary(c, ar, save, label)
		return
	}

	result, apiErr := buffered(build)(c)
	if apiErr != nil {
		middleware.RefundQuota(c)
		apiErr.send(c)
		return
	}
	if save {
		title := result.Filename
		if label != "" {
			title = titleFromText(result.Text, label)
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
	user := middleware.CurrentUser(c)
	document := models.Document{
		Filename:   title,
		Summary:    result.Summary,
		UserID:     user.ID,
		Content:    result.Text,
		HasContent: result.Text != "",
	}
	if err := initializers.DB.Create(&document).Error; err != nil {
		_ = c.Error(err)
		return
	}
	storeFiles(user.ID, document.ID, result.files)
	indexAfterSave(middleware.RequestIDFrom(c), document)
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// NextCursorHeader carries the cursor for the next page; it is empty on the last page.
	NextCursorHeader = "X-Next-Cursor"
)

// parsePage reads ?limit= and ?before= (a document id; only older documents are returned).
func parsePage(limit, before string) (int, uint64, *apiError) {
	size := defaultPageSize
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 {
			return 0, 0, &apiError{http.StatusBadRequest, "limit must be a positive number."}
		}
		size = min(n, maxPageSize)
	}

	var cursor uint64
	if before != "" {
		n, err := strconv.ParseUint(before, 10, 64)
		if err != nil {
			return 0, 0, &apiError{http.StatusBadRequest, "before must be a document id."}
		}
		cursor = n
	}
	return size, cursor, nil
}

// ListDocuments returns the signed-in user's documents, newest first, one page at a time.
// Pass the X-Next-Cursor response header back as ?before= to get the following page.
func ListDocuments(c *gin.Context) {
	user := middleware.CurrentUser(c)

	size, before, apiErr := parsePage(c.Query("limit"), c.Query("before"))
	if apiErr != nil {
		apiErr.send(c)
		return
	}

	// Content can be hundreds of KB per document and is never shown in the list.
	query := initializers.DB.Omit("content").Where("user_id = ?", user.ID)
	if before > 0 {
		query = query.Where("id < ?", before)
	}

	// Fetch one extra row to learn whether another page exists without a second count query.
	var documents []models.Document
	if err := query.Order("id DESC").Limit(size + 1).Find(&documents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load documents"})
		return
	}

	if err := attachFiles(documents); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load documents"})
		return
	}

	next := ""
	if len(documents) > size {
		documents = documents[:size]
		next = strconv.FormatUint(uint64(documents[size-1].ID), 10)
	}
	c.Header(NextCursorHeader, next)
	c.JSON(http.StatusOK, documents)
}

func DeleteDocument(c *gin.Context) {
	user := middleware.CurrentUser(c)

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

	// The document is only soft-deleted, so its original PDFs (which the database would remove along
	// with a hard delete) are removed explicitly, together with the document, or neither.
	err = initializers.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM document_files WHERE document_id = ?", document.ID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM document_passages WHERE document_id = ?", document.ID).Error; err != nil {
			return err
		}
		return tx.Delete(&document).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document"})
		return
	}
	c.Status(http.StatusOK)
}
