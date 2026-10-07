package controllers

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// Bounds on what reaches the browser, whatever the AI service sends.
const (
	maxProofSentences       = 400
	maxProofPassages        = 3
	maxProofSentenceRunes   = 2_000
	maxProofMissingNumbers  = 20
	proofSupportStrong      = "strong"
	proofSupportWeak        = "weak"
	proofSupportNone        = "none"
	proofKindClaim          = "claim"
	proofKindHeading        = "heading"
	defaultProofUnavailable = "This document was saved before checking was available. Summarize it again to check it."
)

type proofPassage struct {
	ID       int     `json:"id"`
	Text     string  `json:"text"`
	Page     *int    `json:"page"`
	PageEnd  *int    `json:"pageEnd"`
	Document string  `json:"document,omitempty"`
	Coverage float64 `json:"coverage"`
}

type proofSentence struct {
	Text     string  `json:"text"`
	Kind     string  `json:"kind"`
	Support  *string `json:"support"`
	Coverage float64 `json:"coverage"`
	// Numbers in the summary that the document never mentions, and ones it has but elsewhere.
	MissingNumbers   []string       `json:"missingNumbers"`
	ElsewhereNumbers []string       `json:"elsewhereNumbers"`
	Passages         []proofPassage `json:"passages"`
}

// proofResult is how well each sentence of a summary is backed by the document it came from.
type proofResult struct {
	Sentences  []proofSentence `json:"sentences"`
	Claims     int             `json:"claims"`
	Found      int             `json:"found"`
	Partly     int             `json:"partly"`
	NotFound   int             `json:"notFound"`
	Verifiable bool            `json:"verifiable"`
}

// CheckDocumentSummary compares every sentence of one of the signed-in user's saved summaries with
// the document's text. It is plain text matching in the AI service (no language model), so it is
// not counted against the daily quota.
func CheckDocumentSummary(c *gin.Context) {
	user := middleware.CurrentUser(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document id"})
		return
	}

	// Scoping the query to the user means other users' documents look like they don't exist.
	var document models.Document
	if err := initializers.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&document).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	if !document.HasContent || document.Content == "" || document.Summary == "" {
		c.JSON(http.StatusConflict, gin.H{"error": defaultProofUnavailable})
		return
	}

	payload, err := json.Marshal(gin.H{"summary": document.Summary, "text": document.Content})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/proof"), bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the request."})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	prepareAIRequest(c, req)

	body, apiErr := callAIService(req)
	if apiErr != nil {
		apiErr.send(c)
		return
	}
	var result proofResult
	if err := json.Unmarshal(body, &result); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "The AI service returned an unreadable result."})
		return
	}
	c.JSON(http.StatusOK, cleanProof(result))
}

// cleanProof bounds and normalizes a proof result: unknown kinds and support levels are dropped,
// sizes are capped, and lists are never null.
func cleanProof(in proofResult) proofResult {
	out := proofResult{
		Sentences: make([]proofSentence, 0, len(in.Sentences)),
		Claims:    max(in.Claims, 0), Found: max(in.Found, 0), Partly: max(in.Partly, 0), NotFound: max(in.NotFound, 0),
		Verifiable: in.Verifiable,
	}
	for _, s := range in.Sentences {
		if len(out.Sentences) >= maxProofSentences {
			break
		}
		if s.Kind != proofKindClaim && s.Kind != proofKindHeading {
			continue
		}
		s.Text = truncateRunes(s.Text, maxProofSentenceRunes)
		if s.Text == "" {
			continue
		}
		if s.Kind == proofKindHeading || s.Support == nil || !validSupport(*s.Support) {
			s.Support = nil
		}
		if s.Coverage < 0 || s.Coverage > 1 {
			s.Coverage = 0
		}
		s.MissingNumbers = boundedList(s.MissingNumbers)
		s.ElsewhereNumbers = boundedList(s.ElsewhereNumbers)
		s.Passages = cleanProofPassages(s.Passages)
		out.Sentences = append(out.Sentences, s)
	}
	return out
}

// boundedList caps a list of short strings and makes sure it is never null in the JSON.
func boundedList(in []string) []string {
	if in == nil {
		return []string{}
	}
	if len(in) > maxProofMissingNumbers {
		return in[:maxProofMissingNumbers]
	}
	return in
}

func validSupport(s string) bool {
	return s == proofSupportStrong || s == proofSupportWeak || s == proofSupportNone
}

func cleanProofPassages(in []proofPassage) []proofPassage {
	out := make([]proofPassage, 0, len(in))
	for _, p := range in {
		if len(out) >= maxProofPassages {
			break
		}
		if p.ID <= 0 || p.Text == "" {
			continue
		}
		if utf8.RuneCountInString(p.Text) > maxSourceText {
			p.Text = truncateRunes(p.Text, maxSourceText) + "…"
		}
		if p.Page != nil && *p.Page < 1 {
			p.Page, p.PageEnd = nil, nil
		}
		if p.PageEnd != nil && (p.Page == nil || *p.PageEnd < *p.Page) {
			p.PageEnd = p.Page
		}
		if p.Coverage < 0 || p.Coverage > 1 {
			p.Coverage = 0
		}
		out = append(out, p)
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
