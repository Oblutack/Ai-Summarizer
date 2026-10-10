package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"ai-summarizer/go-api/middleware"

	"github.com/gin-gonic/gin"
)

// Limits on what can be asked for: keep in step with python-ai-service/extraction.py.
const (
	maxExtractFields      = 20
	maxExtractNameChars   = 60
	maxExtractDescription = 200
	maxExtractValue       = 400
	maxExtractQuote       = 300
	maxExtractReason      = 200
)

var extractTypes = []string{"text", "number", "date", "amount", "list"}

type extractField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// extractResult is one field's outcome. The AI service's reply is read into this and written out again, so only these
// values reach a browser, each in the range the interface expects.
type extractResult struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Quote    string `json:"quote"`
	Page     *int   `json:"page"`
	Found    bool   `json:"found"`
	Verified bool   `json:"verified"`
	Reason   string `json:"reason"`
}

// plainLine trims a one-line text, replaces runs of whitespace with one space and drops control characters.
func plainLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// cleanExtractFields checks the fields a person asked for and tidies them. The message of the error is safe to show.
func cleanExtractFields(raw []extractField) ([]extractField, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("Choose at least one field to extract.")
	}
	if len(raw) > maxExtractFields {
		return nil, fmt.Errorf("At most %d fields can be extracted at once.", maxExtractFields)
	}
	seen := map[string]bool{}
	out := make([]extractField, 0, len(raw))
	for _, f := range raw {
		name := plainLine(f.Name)
		if name == "" {
			return nil, fmt.Errorf("Every field needs a name.")
		}
		if utf8.RuneCountInString(name) > maxExtractNameChars {
			return nil, fmt.Errorf("A field name can be at most %d characters.", maxExtractNameChars)
		}
		if seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("The field “%s” is listed twice.", name)
		}
		seen[strings.ToLower(name)] = true
		kind := strings.ToLower(strings.TrimSpace(f.Type))
		if kind == "" {
			kind = "text"
		}
		if !contains(extractTypes, kind) {
			return nil, fmt.Errorf("Unknown field type.")
		}
		out = append(out, extractField{Name: name, Description: clip(plainLine(f.Description), maxExtractDescription), Type: kind})
	}
	return out, nil
}

// ExtractFromDocument pulls named fields (an invoice number, a total, a notice period) out of one of the user's
// documents. Each value comes with the exact quote it was read from, and the AI service has already checked that the
// quote is in the document and the value is in the quote, so a person can tell which cells to trust.
// Nothing is saved: the table is worked out when it is asked for.
func ExtractFromDocument(c *gin.Context) {
	id, ok := documentIDParam(c)
	if !ok {
		middleware.RefundQuota(c)
		return
	}
	var body struct {
		Fields   []extractField `json:"fields"`
		Language string         `json:"language"`
	}
	if !bindJSON(c, &body) {
		middleware.RefundQuota(c)
		return
	}
	fields, err := cleanExtractFields(body.Fields)
	if err != nil {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Language != "" && !contains(summaryLanguages, body.Language) {
		middleware.RefundQuota(c)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported language."})
		return
	}
	document, ok := ownedDocument(c, id, true)
	if !ok {
		middleware.RefundQuota(c)
		return
	}
	if !document.HasContent || strings.TrimSpace(document.Content) == "" {
		middleware.RefundQuota(c)
		c.JSON(http.StatusConflict, gin.H{"error": "This document has no saved text to extract from (it was saved before the text was kept)."})
		return
	}

	result, apiErr := postExtraction(c, document.Filename, document.Content, fields, body.Language)
	if apiErr != nil {
		middleware.RefundQuota(c)
		apiErr.send(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

// postExtraction asks the AI service for the fields of a text and returns the result in the shape that may be sent
// on, for exactly the fields that were asked for.
func postExtraction(c *gin.Context, name, text string, fields []extractField, language string) (gin.H, *apiError) {
	reply, apiErr := postToAI(c, "/extract", gin.H{"name": name, "text": text, "fields": fields, "language": language})
	if apiErr != nil {
		return nil, apiErr
	}
	var raw struct {
		Fields []extractResult `json:"fields"`
		// Suspicious: the document has sentences written for an AI ("ignore your instructions...").
		Suspicious bool `json:"suspicious"`
	}
	if err := json.Unmarshal(reply, &raw); err != nil || len(raw.Fields) != len(fields) {
		return nil, &apiError{http.StatusBadGateway, "The AI service returned an unreadable result."}
	}
	results := make([]extractResult, 0, len(raw.Fields))
	for i, r := range raw.Fields {
		// the names and types are the ones that were asked for, whatever the reply says
		r.Name, r.Type = fields[i].Name, fields[i].Type
		r.Value, r.Quote, r.Reason = clip(plainLine(r.Value), maxExtractValue), clip(plainLine(r.Quote), maxExtractQuote), clip(plainLine(r.Reason), maxExtractReason)
		if r.Page != nil && *r.Page < 1 {
			r.Page = nil
		}
		if !r.Found {
			r.Value, r.Quote, r.Verified, r.Reason = "", "", false, ""
		}
		r.Verified = r.Verified && r.Found
		results = append(results, r)
	}
	return gin.H{"name": name, "fields": results, "suspicious": raw.Suspicious}, nil
}
