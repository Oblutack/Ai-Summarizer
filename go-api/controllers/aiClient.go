package controllers

import (
	"ai-summarizer/go-api/metrics"
	"ai-summarizer/go-api/middleware"
	"ai-summarizer/go-api/models"
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
	MaxPDFBytes   = 10 << 20 // 10 MB per PDF
	MaxMultiBytes = 25 << 20 // 25 MB across all PDFs in one multi-document request
	MaxTextBytes  = 1 << 20  // 1 MB request body for pasted text
	maxFiles      = 5
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

// summaryResponse is what clients receive.
type summaryResponse struct {
	Filename string `json:"filename,omitempty"`
	Summary  string `json:"summary"`
}

// aiSummary is the AI service's reply. Text is the extracted source text, kept server-side
// so the document can be chatted with later.
type aiSummary struct {
	Filename string `json:"filename"`
	Summary  string `json:"summary"`
	Text     string `json:"text"`

	// files are the uploaded PDFs behind this summary (set by the gateway, not the AI service).
	files []storedFile
}

func (s *aiSummary) response() summaryResponse {
	return summaryResponse{Filename: s.Filename, Summary: s.Summary}
}

type summaryOptions struct {
	Words    int
	Pages    int
	Style    string
	Language string
	// Instructions are the signed-in user's standing preferences; empty for anonymous summaries.
	Instructions string
}

// maxInstructionRunes is the longest set of standing preferences a user can keep.
const maxInstructionRunes = 500

// instructionsOf returns the signed-in user's standing summary preferences, if there is a signed-in user.
func instructionsOf(c *gin.Context) string {
	if v, ok := c.Get(middleware.UserKey); ok {
		if user, ok := v.(models.User); ok {
			return user.CustomInstructions
		}
	}
	return ""
}

// apiError is a failure that can be reported to the client as-is.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) send(c *gin.Context) {
	c.JSON(e.Status, gin.H{"error": e.Message})
}

// parseSummaryParams validates the summary options; empty values fall back to defaults.
func parseSummaryParams(wordCount, pageLimit, style, language string) (summaryOptions, *apiError) {
	opts := summaryOptions{Words: defaultWords, Style: defaultStyle, Language: defaultLanguage}

	if wordCount != "" {
		n, err := strconv.Atoi(wordCount)
		if err != nil || n < minWords || n > maxWords {
			return opts, &apiError{http.StatusBadRequest, fmt.Sprintf("Word count must be between %d and %d.", minWords, maxWords)}
		}
		opts.Words = n
	}
	if pageLimit != "" {
		n, err := strconv.Atoi(pageLimit)
		if err != nil || n < 0 || n > maxPageLimit {
			return opts, &apiError{http.StatusBadRequest, fmt.Sprintf("Page limit must be between 0 and %d.", maxPageLimit)}
		}
		opts.Pages = n
	}
	if style != "" {
		if !contains(summaryStyles, style) {
			return opts, &apiError{http.StatusBadRequest, "Unknown summary style."}
		}
		opts.Style = style
	}
	if language != "" {
		if !contains(summaryLanguages, language) {
			return opts, &apiError{http.StatusBadRequest, "Unsupported language."}
		}
		opts.Language = language
	}
	return opts, nil
}

func (o summaryOptions) fields() map[string]string {
	fields := map[string]string{
		"word_count": strconv.Itoa(o.Words),
		"page_limit": strconv.Itoa(o.Pages),
		"style":      o.Style,
		"language":   o.Language,
	}
	if o.Instructions != "" {
		fields["instructions"] = o.Instructions
	}
	return fields
}

// prepareAIRequest adds what the AI service expects on every call from the gateway: the request id,
// so its logs can be matched to ours, and the shared secret (AI_SERVICE_TOKEN) when one is set.
// The secret matters when the AI service is reachable from the internet, as on most hosting
// platforms' free tiers: without it anyone who found the URL could spend the model quota.
func prepareAIRequest(c *gin.Context, req *http.Request) {
	setAIHeaders(req, middleware.RequestIDFrom(c))
}

// setAIHeaders is prepareAIRequest for work that outlives a request (indexing a saved document),
// where only the request id is at hand.
func setAIHeaders(req *http.Request, requestID string) {
	if requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	if token := os.Getenv("AI_SERVICE_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// aiServiceURL is the full URL of an AI service path. AI_SERVICE_URL may omit the scheme (hosting
// platforms often hand out just "host:port" for private networking); http:// is assumed then.
func aiServiceURL(path string) string {
	base := strings.TrimRight(os.Getenv("AI_SERVICE_URL"), "/")
	if base != "" && !strings.Contains(base, "://") {
		base = "http://" + base
	}
	return base + path
}

// callAIService performs the request and returns the raw body of a successful reply.
// Failures from the AI service are mapped to client-safe errors.
func callAIService(req *http.Request) ([]byte, *apiError) {
	started := time.Now()
	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		metrics.AIRequest(req.URL.Path, "unavailable", time.Since(started))
		return nil, &apiError{http.StatusGatewayTimeout, "The AI service is unavailable or took too long to respond."}
	}
	defer func() { _ = resp.Body.Close() }()
	metrics.AIRequest(req.URL.Path, aiOutcome(resp.StatusCode), time.Since(started))

	// Summaries echo the source text back (to be stored for chat), so allow more than a summary needs.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, &apiError{http.StatusBadGateway, "Failed to read the AI service response."}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, upstreamError(resp.StatusCode, body)
	}
	return body, nil
}

// aiOutcome is the metrics label for an AI service reply.
func aiOutcome(status int) string {
	if status == http.StatusOK {
		return "ok"
	}
	return "upstream_error"
}

// upstreamError maps a failed AI service reply to a client-safe error: its own 4xx messages are
// passed through, anything else becomes a generic 502 so internals never leak.
func upstreamError(status int, body []byte) *apiError {
	var detail struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(body, &detail)
	// 4xx are the caller's problem and 503 is the AI service's own "busy" message; both are
	// written by us, so they are safe to show. Any other 5xx stays generic.
	if (status >= 400 && status < 500 || status == http.StatusServiceUnavailable) && detail.Detail != "" {
		return &apiError{status, detail.Detail}
	}
	return &apiError{http.StatusBadGateway, "The AI service failed to produce a response."}
}

func callForSummary(req *http.Request) (*aiSummary, *apiError) {
	body, apiErr := callAIService(req)
	if apiErr != nil {
		return nil, apiErr
	}
	var out aiSummary
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.Summary) == "" {
		return nil, &apiError{http.StatusBadGateway, "The AI service returned an empty summary."}
	}
	return &out, nil
}

// multipartRequest builds a POST to the AI service with form fields and PDF files
// (sent under fileField, once per file).
func multipartRequest(c *gin.Context, path string, fields map[string]string, fileField string, files []*multipart.FileHeader) (*aiRequest, *apiError) {
	prepErr := &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	var kept []storedFile

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, prepErr
		}
	}
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			return nil, &apiError{http.StatusBadRequest, "Could not read the uploaded file."}
		}
		data, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			return nil, prepErr
		}
		part, err := w.CreateFormFile(fileField, fh.Filename)
		if err == nil {
			_, err = part.Write(data)
		}
		if err != nil {
			return nil, prepErr
		}
		kept = append(kept, storedFile{Name: fh.Filename, Data: data}) // saved with the summary, if it is saved
	}
	if err := w.Close(); err != nil {
		return nil, prepErr
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL(path), &buf)
	if err != nil {
		return nil, prepErr
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	prepareAIRequest(c, req)
	return &aiRequest{req: req, files: kept}, nil
}

// aiRequest is a prepared call to the AI service. sourceText is set when the caller already
// holds the source text (pasted text); for PDFs the AI service returns it.
type aiRequest struct {
	req        *http.Request
	sourceText string
	// files are the uploaded PDFs, kept so they can be stored with the summary.
	files []storedFile
}

// summaryBuilder validates a client request and prepares the matching AI service call.
// With stream set, the AI service replies with server-sent events instead of one JSON body.
type summaryBuilder func(c *gin.Context, stream bool) (*aiRequest, *apiError)

func withStream(path string, stream bool) string {
	if stream {
		return path + "?stream=true"
	}
	return path
}

// buffered runs a builder and waits for the complete summary.
func buffered(build summaryBuilder) func(*gin.Context) (*aiSummary, *apiError) {
	return func(c *gin.Context) (*aiSummary, *apiError) {
		ar, apiErr := build(c, false)
		if apiErr != nil {
			return nil, apiErr
		}
		result, apiErr := callForSummary(ar.req)
		if apiErr != nil {
			return nil, apiErr
		}
		if ar.sourceText != "" {
			result.Text = ar.sourceText
		}
		result.files = ar.files
		return result, nil
	}
}

func buildFileRequest(c *gin.Context, stream bool) (*aiRequest, *apiError) {
	opts, perr := parseSummaryParams(c.PostForm("wordCount"), c.PostForm("pageLimit"), c.PostForm("style"), c.PostForm("language"))
	if perr != nil {
		return nil, perr
	}
	opts.Instructions = instructionsOf(c)

	file, err := c.FormFile("file")
	if err != nil {
		return nil, &apiError{http.StatusBadRequest, "A PDF file is required (max 10 MB)."}
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".pdf") {
		return nil, &apiError{http.StatusBadRequest, "Only PDF files are supported."}
	}

	return multipartRequest(c, withStream("/summarize", stream), opts.fields(), "file", []*multipart.FileHeader{file})
}

func buildFilesRequest(c *gin.Context, stream bool) (*aiRequest, *apiError) {
	opts, perr := parseSummaryParams(c.PostForm("wordCount"), c.PostForm("pageLimit"), c.PostForm("style"), c.PostForm("language"))
	if perr != nil {
		return nil, perr
	}
	opts.Instructions = instructionsOf(c)

	form, err := c.MultipartForm()
	if err != nil {
		return nil, &apiError{http.StatusBadRequest, "Invalid upload."}
	}
	files := form.File["files"]
	if len(files) == 0 || len(files) > maxFiles {
		return nil, &apiError{http.StatusBadRequest, fmt.Sprintf("Upload between 1 and %d PDF files.", maxFiles)}
	}
	var total int64
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f.Filename), ".pdf") {
			return nil, &apiError{http.StatusBadRequest, "Only PDF files are supported."}
		}
		if f.Size > MaxPDFBytes {
			return nil, &apiError{http.StatusRequestEntityTooLarge, f.Filename + " is too large (max 10 MB per file)."}
		}
		total += f.Size
	}
	if total > MaxMultiBytes {
		return nil, &apiError{http.StatusRequestEntityTooLarge, "The files are too large together (max 25 MB)."}
	}

	return multipartRequest(c, withStream("/summarize-multiple", stream), opts.fields(), "files", files)
}

func buildTextRequest(c *gin.Context, stream bool) (*aiRequest, *apiError) {
	opts, perr := parseSummaryParams(c.Query("wordCount"), c.Query("pageLimit"), c.Query("style"), c.Query("language"))
	if perr != nil {
		return nil, perr
	}
	opts.Instructions = instructionsOf(c)

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
	q := url.Values{}
	for k, v := range opts.fields() {
		q.Set(k, v)
	}
	if stream {
		q.Set("stream", "true")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, aiServiceURL("/summarize-text?"+q.Encode()), bytes.NewReader(body))
	if err != nil {
		return nil, &apiError{http.StatusInternalServerError, "Failed to prepare the request."}
	}
	req.Header.Set("Content-Type", "application/json")
	prepareAIRequest(c, req)
	return &aiRequest{req: req, sourceText: payload.Text}, nil
}
