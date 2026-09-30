package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	MaxPDFBytes   = 10 << 20 // 10 MB
	MaxTextBytes  = 1 << 20  // 1 MB request body for pasted text
	maxTextChars  = 200_000
	defaultWords  = 150
	minWords      = 50
	maxWords      = 1000
	maxPageLimit  = 20
	aiCallTimeout = 3 * time.Minute
)

var aiHTTPClient = &http.Client{Timeout: aiCallTimeout}

type TextPayload struct {
	Text string `json:"text"`
}

type summaryResponse struct {
	Filename string `json:"filename,omitempty"`
	Summary  string `json:"summary"`
}

// apiError is a failure that can be reported to the client as-is.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) send(c *gin.Context) {
	c.JSON(e.Status, gin.H{"error": e.Message})
}

// parseSummaryParams validates the length options; empty values fall back to defaults.
func parseSummaryParams(wordCount, pageLimit string) (int, int, *apiError) {
	words, pages := defaultWords, 0

	if wordCount != "" {
		n, err := strconv.Atoi(wordCount)
		if err != nil || n < minWords || n > maxWords {
			return 0, 0, &apiError{http.StatusBadRequest, fmt.Sprintf("Word count must be between %d and %d.", minWords, maxWords)}
		}
		words = n
	}
	if pageLimit != "" {
		n, err := strconv.Atoi(pageLimit)
		if err != nil || n < 0 || n > maxPageLimit {
			return 0, 0, &apiError{http.StatusBadRequest, fmt.Sprintf("Page limit must be between 0 and %d.", maxPageLimit)}
		}
		pages = n
	}
	return words, pages, nil
}

func aiServiceURL(path string) string {
	return strings.TrimRight(os.Getenv("AI_SERVICE_URL"), "/") + path
}

// callAIService performs the request and decodes a successful reply. Failures from the
// AI service are mapped to client-safe errors.
func callAIService(req *http.Request) (*summaryResponse, *apiError) {
	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return nil, &apiError{http.StatusGatewayTimeout, "The AI service is unavailable or took too long to respond."}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, &apiError{http.StatusBadGateway, "Failed to read the AI service response."}
	}

	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(body, &detail)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && detail.Detail != "" {
			return nil, &apiError{resp.StatusCode, detail.Detail}
		}
		return nil, &apiError{http.StatusBadGateway, "The AI service failed to produce a summary."}
	}

	var out summaryResponse
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.Summary) == "" {
		return nil, &apiError{http.StatusBadGateway, "The AI service returned an empty summary."}
	}
	return &out, nil
}

func summarizeFile(c *gin.Context) (*summaryResponse, *apiError) {
	words, pages, perr := parseSummaryParams(c.PostForm("wordCount"), c.PostForm("pageLimit"))
	if perr != nil {
		return nil, perr
	}

	file, err := c.FormFile("file")
	if err != nil {
		return nil, &apiError{http.StatusBadRequest, "A PDF file is required (max 10 MB)."}
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".pdf") {
		return nil, &apiError{http.StatusBadRequest, "Only PDF files are supported."}
	}
	src, err := file.Open()
	if err != nil {
		return nil, &apiError{http.StatusBadRequest, "Could not read the uploaded file."}
	}
	defer src.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("word_count", strconv.Itoa(words))
	_ = w.WriteField("page_limit", strconv.Itoa(pages))
	part, err := w.CreateFormFile("file", file.Filename)
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	if _, err := io.Copy(part, src); err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	if err := w.Close(); err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/summarize"), &buf)
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return callAIService(req)
}

func summarizeText(c *gin.Context) (*summaryResponse, *apiError) {
	words, pages, perr := parseSummaryParams(c.Query("wordCount"), c.Query("pageLimit"))
	if perr != nil {
		return nil, perr
	}

	var payload TextPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &apiError{http.StatusBadRequest, "Invalid request body."}
	}
	if strings.TrimSpace(payload.Text) == "" {
		return nil, &apiError{http.StatusBadRequest, "Text is required."}
	}
	if utf8.RuneCountInString(payload.Text) > maxTextChars {
		return nil, &apiError{http.StatusRequestEntityTooLarge, fmt.Sprintf("Text is too long (max %d characters).", maxTextChars)}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	q := url.Values{"word_count": {strconv.Itoa(words)}, "page_limit": {strconv.Itoa(pages)}}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/summarize-text?"+q.Encode()), bytes.NewReader(body))
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	req.Header.Set("Content-Type", "application/json")
	return callAIService(req)
}
