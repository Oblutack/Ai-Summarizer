package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterBurstAndRefill(t *testing.T) {
	l := NewRateLimiter(60, 2) // 1 token/second, burst of 2
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }

	firstOK := l.Allow("a")
	secondOK := l.Allow("a")
	if !firstOK || !secondOK {
		t.Fatal("burst requests should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("request over burst should be rejected")
	}
	if !l.Allow("b") {
		t.Fatal("other keys must have their own bucket")
	}

	now = now.Add(1100 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("token should refill after a second")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(NewRateLimiter(1, 1)))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := []int{}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests {
		t.Fatalf("got %v, want [200 429]", codes)
	}
}

func TestMaxBodyRejectsLargeRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MaxBody(10))
	r.POST("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.ContentLength = 11
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", w.Code)
	}
}
