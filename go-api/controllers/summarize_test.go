package controllers

import (
	"net/http"
	"net/http/httptest"
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
