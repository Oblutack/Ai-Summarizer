package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	minOverviewDocuments = 2
	maxOverviewDocuments = 20
	// Each document brings its saved summary, which is short already; this only guards against a huge one.
	maxOverviewSummaryRunes = 4000
	defaultOverviewWords    = 300
)

// overviewRequest asks for one briefing on the documents of a collection (the documents with a tag).
type overviewRequest struct {
	Tag string `json:"tag"`
}

type overviewDocument struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// CollectionOverview writes one briefing on a collection: what its documents are about together, where
// they agree and where they differ. It is written from the saved summaries (short, so it is quick and
// cheap) and is not saved: it is shown, can be copied, and is made again when asked. Like any summary it
// counts against the daily allowance, and is refunded if it fails.
func CollectionOverview(c *gin.Context) {
	respond(c, buildOverviewRequest, false, "")
}

func buildOverviewRequest(c *gin.Context, stream bool) (*aiRequest, *apiError) {
	opts, perr := parseSummaryParams(c.Query("wordCount"), "", c.Query("style"), c.Query("language"))
	if perr != nil {
		return nil, perr
	}
	if c.Query("wordCount") == "" {
		opts.Words = defaultOverviewWords
	}
	opts.Instructions = instructionsOf(c)

	var body overviewRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		return nil, &apiError{http.StatusBadRequest, "Invalid request body."}
	}
	tag := strings.ToLower(cleanLine(body.Tag))
	if tag == "" || utf8.RuneCountInString(tag) > maxTagRunes {
		return nil, &apiError{http.StatusBadRequest, "Choose a collection (a tag) to write an overview of."}
	}

	user := middleware.CurrentUser(c)
	wanted, err := json.Marshal([]string{tag})
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	var rows []models.Document
	err = initializers.DB.WithContext(c.Request.Context()).
		Select("id", "filename", "summary").
		Where("user_id = ? AND summary <> '' AND tags @> ?::jsonb", user.ID, string(wanted)).
		Order("id DESC").Limit(maxOverviewDocuments).Find(&rows).Error
	if err != nil {
		_ = c.Error(err)
		return nil, &apiError{http.StatusInternalServerError, "Failed to load the documents of that collection."}
	}
	if len(rows) < minOverviewDocuments {
		return nil, &apiError{http.StatusBadRequest, "An overview needs at least two documents with that tag."}
	}

	// Oldest first, so the briefing reads in the order the documents were saved.
	docs := make([]overviewDocument, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		docs = append(docs, overviewDocument{Name: rows[i].Filename, Text: truncateRunes(rows[i].Summary, maxOverviewSummaryRunes)})
	}
	payload, err := json.Marshal(gin.H{"name": tag, "documents": docs})
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}

	q := url.Values{}
	for k, v := range opts.fields() {
		if k != "page_limit" { // an overview has no page limit
			q.Set(k, v)
		}
	}
	if stream {
		q.Set("stream", "true")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/overview?"+q.Encode()), bytes.NewReader(payload))
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	req.Header.Set("Content-Type", "application/json")
	prepareAIRequest(c, req)
	return &aiRequest{req: req}, nil
}
