package controllers

import (
	"ai-summarizer/go-api/middleware"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func routerWithRequestID() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.POST("/public/summarize-text", PublicSummarizeText)
	return r
}

func TestRequestIDIsForwardedToTheAIService(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(middleware.RequestIDHeader)
		_, _ = w.Write([]byte(`{"summary":"ok"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/public/summarize-text", strings.NewReader(`{"text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "trace-0123456789")
	routerWithRequestID().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if seen != "trace-0123456789" {
		t.Fatalf("AI service saw request id %q, want the caller's id", seen)
	}
	if w.Header().Get(middleware.RequestIDHeader) != "trace-0123456789" {
		t.Fatal("the client should get the same id back")
	}
}

func TestAIServiceBusyMessagePassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"detail":"The AI service is busy right now. Please try again in a minute."}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	w := postText(summarizeRouter(), "", `{"text":"hello"}`)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "busy right now") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestOtherServerErrorsStayGeneric(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"Traceback: secret internals"}`))
		}))
		t.Setenv("AI_SERVICE_URL", srv.URL)

		w := postText(summarizeRouter(), "", `{"text":"hello"}`)
		srv.Close()
		if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "secret") {
			t.Errorf("status %d leaked or mis-mapped: %d %s", status, w.Code, w.Body.String())
		}
	}
}
