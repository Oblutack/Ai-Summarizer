package middleware

import (
	"ai-summarizer/go-api/models"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func logRouter(buf *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	r := gin.New()
	r.Use(RequestID(), AccessLog(logger), Recover(logger))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "hello") })
	r.GET("/echo-id", func(c *gin.Context) { c.String(http.StatusOK, RequestIDFrom(c)) })
	r.GET("/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	r.GET("/panic", func(c *gin.Context) { panic("something exploded") })
	r.GET("/users/:id", func(c *gin.Context) { c.String(http.StatusOK, "x") })
	r.GET("/me", func(c *gin.Context) {
		c.Set("user", models.User{Model: gorm.Model{ID: 42}})
		c.String(http.StatusOK, "x")
	})
	return r
}

func do(r *gin.Engine, path string, header ...string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if len(header) == 2 {
		req.Header.Set(header[0], header[1])
	}
	r.ServeHTTP(w, req)
	return w
}

func lastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var out map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &out); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	return out
}

func TestRequestIDIsGeneratedAndReturned(t *testing.T) {
	var buf bytes.Buffer
	w := do(logRouter(&buf), "/echo-id")
	id := w.Header().Get(RequestIDHeader)
	if len(id) != 16 || w.Body.String() != id {
		t.Fatalf("header %q, handler saw %q", id, w.Body.String())
	}
	if id2 := do(logRouter(&buf), "/echo-id").Header().Get(RequestIDHeader); id2 == id {
		t.Fatal("ids must differ between requests")
	}
}

func TestWellFormedIncomingRequestIDIsReused(t *testing.T) {
	var buf bytes.Buffer
	w := do(logRouter(&buf), "/echo-id", RequestIDHeader, "trace-abc_123.xyz")
	if w.Header().Get(RequestIDHeader) != "trace-abc_123.xyz" || w.Body.String() != "trace-abc_123.xyz" {
		t.Fatalf("incoming id not reused: %q / %q", w.Header().Get(RequestIDHeader), w.Body.String())
	}
}

func TestUnsafeIncomingRequestIDsAreReplaced(t *testing.T) {
	for _, bad := range []string{"short", "has spaces in it!", "line\\nbreak-injection", strings.Repeat("a", 200), "<script>alert(1)</script>"} {
		var buf bytes.Buffer
		got := do(logRouter(&buf), "/ok", RequestIDHeader, bad).Header().Get(RequestIDHeader)
		if got == bad || len(got) != 16 {
			t.Errorf("incoming id %q should have been replaced, got %q", bad, got)
		}
	}
}

func TestAccessLogHasTheExpectedFields(t *testing.T) {
	var buf bytes.Buffer
	w := do(logRouter(&buf), "/ok", RequestIDHeader, "req-12345678")
	line := lastLine(t, &buf)

	for key, want := range map[string]any{"msg": "request", "level": "INFO", "request_id": "req-12345678", "method": "GET", "route": "/ok", "status": float64(200), "bytes": float64(w.Body.Len())} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
	for _, key := range []string{"duration_ms", "client_ip", "time"} {
		if _, ok := line[key]; !ok {
			t.Errorf("missing %s", key)
		}
	}
}

func TestAccessLogLevelsFollowTheStatus(t *testing.T) {
	for path, level := range map[string]string{"/ok": "INFO", "/bad": "WARN", "/boom": "ERROR"} {
		var buf bytes.Buffer
		do(logRouter(&buf), path)
		if got := lastLine(t, &buf)["level"]; got != level {
			t.Errorf("%s logged at %v, want %s", path, got, level)
		}
	}
}

func TestAccessLogUsesTheRoutePatternNotTheRawURL(t *testing.T) {
	var buf bytes.Buffer
	do(logRouter(&buf), "/users/12345?token=SECRET-VALUE&wordCount=100")
	line := lastLine(t, &buf)
	if line["route"] != "/users/:id" {
		t.Errorf("route = %v", line["route"])
	}
	if strings.Contains(buf.String(), "SECRET-VALUE") || strings.Contains(buf.String(), "12345") {
		t.Fatalf("query strings and ids must not reach the logs: %s", buf.String())
	}
}

func TestAccessLogIncludesTheUserWhenKnown(t *testing.T) {
	var buf bytes.Buffer
	r := logRouter(&buf)
	do(r, "/me")
	if got := lastLine(t, &buf)["user_id"]; got != float64(42) {
		t.Errorf("user_id = %v", got)
	}
	do(r, "/ok")
	if _, ok := lastLine(t, &buf)["user_id"]; ok {
		t.Error("anonymous requests must not log a user")
	}
}

func TestUnmatchedRoutesAreLoggedWithoutTheirPath(t *testing.T) {
	var buf bytes.Buffer
	do(logRouter(&buf), "/wp-admin/exploit?x=1")
	line := lastLine(t, &buf)
	if line["route"] != "unmatched" || line["status"] != float64(404) {
		t.Errorf("got %v", line)
	}
}

func TestPanicBecomesALoggedJSON500(t *testing.T) {
	var buf bytes.Buffer
	w := do(logRouter(&buf), "/panic", RequestIDHeader, "req-12345678")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "Internal server error") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "exploded") {
		t.Fatal("panic details must not reach the client")
	}
	if !strings.Contains(buf.String(), `"msg":"panic"`) || !strings.Contains(buf.String(), "something exploded") || !strings.Contains(buf.String(), "req-12345678") {
		t.Fatalf("the panic should be logged with its request id: %s", buf.String())
	}
}
