package server

import (
	"net/http"
	"strings"
	"testing"
)

// ---- CSRF: the Origin check ----------------------------------------------------------------

func TestForeignOriginsCannotActAsALoggedInUser(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	user := a.userByEmail(email)
	_ = user

	// The browser attaches the cookie to a request from evil.example; the Origin gives it away.
	evil := *cl
	evil.origin = "https://evil.example"
	for name, call := range map[string]func() reply{
		"logout-all": func() reply { return evil.post("/auth/logout-all", nil) },
		"summarize":  func() reply { return evil.post("/summarize-text", map[string]string{"text": "hello"}) },
		"delete account": func() reply {
			return evil.delete("/account", map[string]string{"confirmEmail": email, "password": goodPass})
		},
		"change password": func() reply {
			return evil.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": "Another-strong-pass-7"})
		},
	} {
		if r := call(); r.Status != http.StatusForbidden {
			t.Errorf("%s from a foreign origin: got %d, want 403", name, r.Status)
		}
	}
	if cl.get("/auth/me").Status != 200 {
		t.Error("the real session must be untouched")
	}
	if a.userByEmail(email).ID == 0 {
		t.Error("the account must still exist")
	}
}

func TestOwnOriginAndNonBrowserClientsAreAllowed(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	if r := cl.post("/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
		t.Errorf("own origin: got %d", r.Status)
	}
	noOrigin := *cl
	noOrigin.origin = ""
	if r := noOrigin.post("/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
		t.Errorf("no Origin header (curl, scripts): got %d", r.Status)
	}
}

func TestForeignOriginsAreNeverGrantedReadAccessEither(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.req(http.MethodGet, "/documents", nil, "Origin", "https://evil.example")
	if r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("a foreign origin must not be granted read access: %v", r.Header)
	}
	if r.Status == 200 {
		t.Error("the CORS layer refuses browser requests from unknown origins outright")
	}
	// requests without an Origin (curl, server to server) are not affected
	if cl.get("/documents").Status != 200 {
		t.Error("same-origin and non-browser reads must work")
	}
}

func TestPublicEndpointsAlsoRejectForeignOrigins(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	cl.origin = "https://evil.example"
	if r := cl.post("/public/summarize-text", map[string]string{"text": "hello"}); r.Status != http.StatusForbidden {
		t.Errorf("got %d, want 403", r.Status)
	}
}

// ---- CORS ----------------------------------------------------------------------------------

func TestCORSAllowsCredentialsOnlyForConfiguredOrigins(t *testing.T) {
	a := newApp(t)
	preflight := func(origin string) reply {
		return a.newClient().req(http.MethodOptions, "/summarize-text", nil,
			"Origin", origin, "Access-Control-Request-Method", "POST", "Access-Control-Request-Headers", "content-type")
	}

	ok := preflight(appOrigin)
	if ok.Header.Get("Access-Control-Allow-Origin") != appOrigin || ok.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("configured origin should be allowed with credentials: %v", ok.Header)
	}
	if ok.Header.Get("Access-Control-Allow-Origin") == "*" {
		t.Error("credentials must never be combined with a wildcard origin")
	}
	bad := preflight("https://evil.example")
	if bad.Header.Get("Access-Control-Allow-Origin") != "" || bad.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("unknown origins must get no CORS grant: %v", bad.Header)
	}
}

func TestCORSExposesTheHeadersTheFrontendNeeds(t *testing.T) {
	a := newApp(t)
	r := a.newClient().req(http.MethodGet, "/healthz", nil, "Origin", appOrigin)
	exposed := r.Header.Get("Access-Control-Expose-Headers")
	for _, h := range []string{"X-Next-Cursor", "X-Request-Id", "X-Quota-Remaining", "Retry-After"} {
		if !strings.Contains(strings.ToLower(exposed), strings.ToLower(h)) {
			t.Errorf("%s should be exposed to the browser, got %q", h, exposed)
		}
	}
}

// ---- security headers ----------------------------------------------------------------------

func TestSecurityHeadersAreOnEveryResponse(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	for _, path := range []string{"/healthz", "/options", "/auth/me", "/does-not-exist"} {
		h := cl.get(path).Header
		for k, want := range map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "no-referrer",
			"Cache-Control":           "no-store",
			"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		} {
			if h.Get(k) != want {
				t.Errorf("%s: %s = %q, want %q", path, k, h.Get(k), want)
			}
		}
		if h.Get("Strict-Transport-Security") != "" {
			t.Errorf("%s: HSTS must not be sent over plain http", path)
		}
	}
}

func TestHSTSIsSentBehindAnHTTPSProxy(t *testing.T) {
	a := newApp(t)
	r := a.newClient().req(http.MethodGet, "/healthz", nil, "X-Forwarded-Proto", "https")
	if !strings.Contains(r.Header.Get("Strict-Transport-Security"), "max-age=") {
		t.Errorf("HSTS missing behind https: %v", r.Header)
	}
}

func TestEveryResponseCarriesARequestID(t *testing.T) {
	a := newApp(t)
	if len(a.newClient().get("/healthz").Header.Get("X-Request-Id")) < 8 {
		t.Error("missing request id")
	}
}

// ---- rate limits -----------------------------------------------------------------------------

func TestSignedInUsersAreRateLimitedPerUserNotPerIP(t *testing.T) {
	rates := generousRates
	rates.SummarizeUserBurst, rates.SummarizeUserPerMinute = 3, 1
	a := newAppWithRates(t, rates)

	alice, _ := a.newUser()
	bob, _ := a.newUser() // same IP as Alice

	got := map[int]int{}
	for i := 0; i < 6; i++ {
		got[alice.post("/summarize-text", map[string]string{"text": "hello world"}).Status]++
	}
	if got[200] != 3 || got[429] != 3 {
		t.Fatalf("Alice should get 3 through then be limited: %v", got)
	}
	if r := bob.post("/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
		t.Errorf("Bob shares Alice's IP but has his own allowance: got %d", r.Status)
	}
}

func TestAnonymousSummariesAreRateLimitedPerIP(t *testing.T) {
	rates := generousRates
	rates.SummarizeIPBurst, rates.SummarizeIPPerMinute = 2, 1
	a := newAppWithRates(t, rates)
	cl := a.newClient()

	got := map[int]int{}
	for i := 0; i < 5; i++ {
		got[cl.post("/public/summarize-text", map[string]string{"text": "hello world"}).Status]++
	}
	if got[200] != 2 || got[429] != 3 {
		t.Errorf("got %v", got)
	}
}

func TestOversizedAuthBodiesAreRejected(t *testing.T) {
	a := newApp(t)
	big := `{"email":"a@b.co","password":"` + strings.Repeat("x", 20_000) + `"}`
	if r := a.newClient().post("/login", big); r.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", r.Status)
	}
}
