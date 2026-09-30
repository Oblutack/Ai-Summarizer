package controllers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func summarizeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/public/summarize-text", PublicSummarizeText)
	return r
}

func postText(r *gin.Engine, query, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/public/summarize-text"+query, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func fakeAIService(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query(); q.Get("word_count") != "150" || q.Get("page_limit") != "0" {
			t.Errorf("unexpected query: %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AI_SERVICE_URL", srv.URL)
}

func TestSummarizeTextSuccess(t *testing.T) {
	fakeAIService(t, http.StatusOK, `{"summary":"short"}`)
	w := postText(summarizeRouter(), "", `{"text":"hello world"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"summary":"short"`) {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestSummarizeTextRejectsBadInput(t *testing.T) {
	r := summarizeRouter()
	cases := map[string]struct{ query, body string }{
		"empty text":    {"", `{"text":"   "}`},
		"bad json":      {"", `not json`},
		"bad wordCount": {"?wordCount=abc", `{"text":"hi"}`},
		"too long":      {"", `{"text":"` + strings.Repeat("a", maxTextChars+1) + `"}`},
	}
	for name, c := range cases {
		if w := postText(r, c.query, c.body); w.Code < 400 || w.Code >= 500 {
			t.Errorf("%s: got %d, want 4xx", name, w.Code)
		}
	}
}

func TestSummarizeTextMapsUpstreamErrors(t *testing.T) {
	fakeAIService(t, http.StatusInternalServerError, `{"detail":"secret internal stack trace"}`)
	w := postText(summarizeRouter(), "", `{"text":"hello"}`)
	if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("5xx from AI service must become a generic 502, got %d %s", w.Code, w.Body.String())
	}
}

func TestSummarizeTextPassesThroughClientErrors(t *testing.T) {
	fakeAIService(t, http.StatusUnprocessableEntity, `{"detail":"No text found"}`)
	w := postText(summarizeRouter(), "", `{"text":"hello"}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "No text found") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestStyleAndLanguageAreForwarded(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"summary":"ok"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	w := postText(summarizeRouter(), "?style=brief&language=Serbian&wordCount=200", `{"text":"hello"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if got.Get("style") != "brief" || got.Get("language") != "Serbian" || got.Get("word_count") != "200" {
		t.Fatalf("unexpected forwarded query: %v", got)
	}
}

func TestSummaryResponseHidesSourceText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"filename":"a.pdf","summary":"ok","text":"the full source"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	r := gin.New()
	r.POST("/m", PublicSummarizeMultiple)
	w := postFiles(r, []string{"a.pdf"})
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "full source") {
		t.Fatalf("source text must not reach the client: %d %s", w.Code, w.Body.String())
	}
}

func postFiles(r *gin.Engine, names []string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, n := range names {
		part, _ := mw.CreateFormFile("files", n)
		_, _ = part.Write([]byte("%PDF-fake"))
	}
	_ = mw.WriteField("style", "bullets")
	_ = mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/m", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	return w
}

func TestSummarizeMultipleForwardsEveryFile(t *testing.T) {
	var files []string
	var style string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		for _, fh := range r.MultipartForm.File["files"] {
			files = append(files, fh.Filename)
		}
		style = r.FormValue("style")
		_, _ = w.Write([]byte(`{"filename":"a.pdf, b.pdf","summary":"combined","text":"x"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/m", PublicSummarizeMultiple)

	w := postFiles(r, []string{"a.pdf", "b.pdf"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "combined") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if strings.Join(files, ",") != "a.pdf,b.pdf" || style != "bullets" {
		t.Fatalf("forwarded files=%v style=%q", files, style)
	}
}

func TestSummarizeMultipleRejectsBadUploads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/m", PublicSummarizeMultiple)

	if w := postFiles(r, []string{"a.pdf", "b.txt"}); w.Code != http.StatusBadRequest {
		t.Errorf("non-pdf: got %d", w.Code)
	}
	if w := postFiles(r, []string{"1.pdf", "2.pdf", "3.pdf", "4.pdf", "5.pdf", "6.pdf"}); w.Code != http.StatusBadRequest {
		t.Errorf("too many files: got %d", w.Code)
	}
	if w := postFiles(r, nil); w.Code != http.StatusBadRequest {
		t.Errorf("no files: got %d", w.Code)
	}
}
