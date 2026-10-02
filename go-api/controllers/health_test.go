package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func get(h gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w
}

func TestHealthzAlwaysOK(t *testing.T) {
	if w := get(Healthz); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok"`) {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestReadyzReportsEachDependency(t *testing.T) {
	up := Check{Name: "database", Run: func(context.Context) error { return nil }}
	down := Check{Name: "ai_service", Run: func(context.Context) error { return errors.New("connection refused") }}

	w := get(Readyz(up))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ready"`) {
		t.Fatalf("all up: got %d %s", w.Code, w.Body.String())
	}

	w = get(Readyz(up, down))
	body := w.Body.String()
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(body, `"database":"ok"`) || !strings.Contains(body, `"ai_service":"unavailable"`) {
		t.Fatalf("one down: got %d %s", w.Code, body)
	}
	if strings.Contains(body, "connection refused") {
		t.Fatal("internal error details must not be exposed to callers")
	}
}

func TestAIServiceCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Setenv("AI_SERVICE_URL", srv.URL)
	if err := AIServiceCheck().Run(context.Background()); err != nil {
		t.Fatalf("healthy service reported as down: %v", err)
	}

	srv.Close()
	if err := AIServiceCheck().Run(context.Background()); err == nil {
		t.Fatal("stopped service should be reported as down")
	}
}

func TestAIServiceCheckRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)
	if err := AIServiceCheck().Run(context.Background()); err == nil {
		t.Fatal("a 500 from the AI service should count as down")
	}
}
