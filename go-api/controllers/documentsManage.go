package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	maxDocumentTitleRunes = 200
	maxTags               = 8
	maxTagRunes           = 30
	maxSearchRunes        = 100
)

// documentIDParam reads :id, answering the client itself when it is not a number.
func documentIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document id"})
		return 0, false
	}
	return uint(id), true
}

// ownedDocument loads one of the signed-in user's documents. Scoping the query to the user means other
// people's documents look exactly like ones that do not exist. The source text is only loaded when asked.
func ownedDocument(c *gin.Context, id uint, withContent bool) (models.Document, bool) {
	user := middleware.CurrentUser(c)
	query := initializers.DB
	if !withContent {
		query = query.Omit("content")
	}
	var document models.Document
	if err := query.Where("id = ? AND user_id = ?", id, user.ID).First(&document).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return document, false
	}
	return document, true
}

// cleanLine trims a one-line text, replaces runs of whitespace with one space and drops control characters.
func cleanLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// normalizeTags lowercases, trims and de-duplicates tags, keeping their order. Empty ones are dropped.
func normalizeTags(in []string) ([]string, *apiError) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		tag := strings.ToLower(cleanLine(raw))
		if tag == "" || seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagRunes {
			return nil, &apiError{http.StatusBadRequest, "A tag can have at most 30 characters."}
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) > maxTags {
		return nil, &apiError{http.StatusBadRequest, "A document can have at most 8 tags."}
	}
	return out, nil
}

type documentUpdate struct {
	Filename *string   `json:"filename"`
	Tags     *[]string `json:"tags"`
}

// UpdateDocument renames a saved document and/or replaces its tags.
func UpdateDocument(c *gin.Context) {
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	var body documentUpdate
	if !bindJSON(c, &body) {
		return
	}
	if body.Filename == nil && body.Tags == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to change."})
		return
	}
	document, ok := ownedDocument(c, id, false)
	if !ok {
		return
	}

	updates := map[string]any{}
	if body.Filename != nil {
		title := cleanLine(*body.Filename)
		if title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The title cannot be empty."})
			return
		}
		if utf8.RuneCountInString(title) > maxDocumentTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The title can have at most 200 characters."})
			return
		}
		updates["filename"] = title
		document.Filename = title
	}
	if body.Tags != nil {
		tags, apiErr := normalizeTags(*body.Tags)
		if apiErr != nil {
			apiErr.send(c)
			return
		}
		updates["tags"] = models.StringList(tags)
		document.Tags = tags
	}
	if err := initializers.DB.Model(&models.Document{}).Where("id = ?", document.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update the document"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ID": document.ID, "Filename": document.Filename, "tags": document.Tags})
}

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ListTags returns the tags the signed-in user has used, most used first.
func ListTags(c *gin.Context) {
	user := middleware.CurrentUser(c)
	tags := []tagCount{}
	err := initializers.DB.Raw(`
		SELECT t AS tag, count(*) AS count
		FROM documents, jsonb_array_elements_text(documents.tags) AS t
		WHERE documents.user_id = ? AND documents.deleted_at IS NULL
		GROUP BY t ORDER BY count DESC, t ASC LIMIT 100`, user.ID).Scan(&tags).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load tags"})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// likePattern turns text into a pattern that matches it literally, anywhere in a value.
func likePattern(s string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + escaped + "%"
}
