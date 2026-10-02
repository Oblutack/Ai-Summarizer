package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeCloudflare stands in for the siteverify endpoint.
type fakeCloudflare struct {
	mu   sync.Mutex
	last url.Values
}

func startFakeCloudflare(t *testing.T, accept func(token string) bool) *fakeCloudflare {
	t.Helper()
	f := &fakeCloudflare{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.last = r.PostForm
		f.mu.Unlock()
		if accept(r.PostForm.Get("response")) {
			_, _ = w.Write([]byte(`{"success":true}`))
		} else {
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	t.Cleanup(srv.Close)

	old := turnstileVerifyURL
	turnstileVerifyURL = srv.URL
	t.Cleanup(func() { turnstileVerifyURL = old })
	return f
}

func turnstileRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", Turnstile(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func postWithToken(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("{}"))
	if token != "" {
		req.Header.Set(TurnstileHeader, token)
	}
	req.RemoteAddr = "203.0.113.9:4000"
	r.ServeHTTP(w, req)
	return w
}

func TestTurnstileIsOffWithoutASecret(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "")
	if w := postWithToken(turnstileRouter(), ""); w.Code != http.StatusOK {
		t.Fatalf("with no secret configured the check must be skipped, got %d", w.Code)
	}
}

func TestTurnstileRequiresAToken(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "secret")
	startFakeCloudflare(t, func(string) bool { return true })
	w := postWithToken(turnstileRouter(), "")
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "turnstile_required") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestTurnstileAcceptsAValidTokenAndSendsTheRightFields(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "the-secret")
	cf := startFakeCloudflare(t, func(tok string) bool { return tok == "good-token" })

	if w := postWithToken(turnstileRouter(), "good-token"); w.Code != http.StatusOK {
		t.Fatalf("valid token rejected: %d %s", w.Code, w.Body.String())
	}
	cf.mu.Lock()
	defer cf.mu.Unlock()
	if cf.last.Get("secret") != "the-secret" || cf.last.Get("response") != "good-token" || cf.last.Get("remoteip") != "203.0.113.9" {
		t.Errorf("Cloudflare was sent %v", cf.last)
	}
}

func TestTurnstileRejectsAnInvalidToken(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "secret")
	startFakeCloudflare(t, func(tok string) bool { return tok == "good-token" })
	w := postWithToken(turnstileRouter(), "forged-token")
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "turnstile_failed") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}

func TestTurnstileFailsClosedWhenCloudflareIsUnreachable(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "secret")
	old := turnstileVerifyURL
	turnstileVerifyURL = "http://127.0.0.1:1" // nothing listens here
	t.Cleanup(func() { turnstileVerifyURL = old })

	w := postWithToken(turnstileRouter(), "any-token")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("an outage must not become a way around the check: got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "127.0.0.1") {
		t.Error("internal addresses must not leak")
	}
}

func TestTurnstileFailsClosedOnAServerError(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET", "secret")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer srv.Close()
	old := turnstileVerifyURL
	turnstileVerifyURL = srv.URL
	t.Cleanup(func() { turnstileVerifyURL = old })

	if w := postWithToken(turnstileRouter(), "tok"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", w.Code)
	}
}

func TestTurnstileTokensAreNotReusableByTheMiddlewareItself(t *testing.T) {
	// Cloudflare enforces single use; the middleware must ask every time rather than cache a pass.
	t.Setenv("TURNSTILE_SECRET", "secret")
	calls := 0
	startFakeCloudflare(t, func(string) bool { calls++; return calls == 1 }) // only the first verification succeeds
	r := turnstileRouter()
	if w := postWithToken(r, "tok"); w.Code != http.StatusOK {
		t.Fatalf("first use: %d", w.Code)
	}
	if w := postWithToken(r, "tok"); w.Code != http.StatusForbidden {
		t.Fatalf("second use of the same token must be verified again and refused: %d", w.Code)
	}
}
