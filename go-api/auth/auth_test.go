package auth

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"ai-summarizer/go-api/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- pure functions (no database) ------------------------------------------------------------

func TestTokensAreUniqueAndLongEnough(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) < 43 { // 32 bytes base64url-encoded
			t.Fatalf("token too short (%d chars): %q", len(tok), tok)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestHashIsStableDeterministicAndNotReversibleByInspection(t *testing.T) {
	first, second, other := HashToken("abc"), HashToken("abc"), HashToken("abd")
	if first != second || first == other {
		t.Fatal("hash should be deterministic and sensitive to the input")
	}
	h := HashToken("secret-token")
	if len(h) != 64 || strings.Contains(h, "secret") {
		t.Errorf("unexpected hash %q", h)
	}
}

func TestCookieConfigFromEnvironment(t *testing.T) {
	cases := []struct {
		secure, sameSite string
		wantSecure       bool
		wantSameSite     http.SameSite
	}{
		{"", "", true, http.SameSiteLaxMode},       // secure by default
		{"false", "", false, http.SameSiteLaxMode}, // explicit opt-out for local http
		{"false", "strict", false, http.SameSiteStrictMode},
		{"false", "none", true, http.SameSiteNoneMode}, // SameSite=None forces Secure: browsers reject it otherwise
		{"true", "NONE", true, http.SameSiteNoneMode},
		{"", "garbage", true, http.SameSiteLaxMode},
	}
	for _, c := range cases {
		t.Setenv("COOKIE_SECURE", c.secure)
		t.Setenv("COOKIE_SAMESITE", c.sameSite)
		got := LoadCookieConfig()
		if got.Secure != c.wantSecure || got.SameSite != c.wantSameSite {
			t.Errorf("secure=%q samesite=%q: got %+v", c.secure, c.sameSite, got)
		}
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	old := Cookie
	Cookie = CookieConfig{Secure: true, SameSite: http.SameSiteNoneMode, Domain: ".example.com"}
	t.Cleanup(func() { Cookie = old })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	SetSessionCookie(c, "tok", time.Now().Add(time.Hour))
	cookie := (&http.Response{Header: w.Header()}).Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode || cookie.Domain != "example.com" || cookie.Path != "/" || cookie.Value != "tok" {
		t.Errorf("%+v", cookie)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	ClearSessionCookie(c)
	cleared := (&http.Response{Header: w.Header()}).Cookies()[0]
	if cleared.MaxAge >= 0 || cleared.Value != "" || !cleared.HttpOnly {
		t.Errorf("clearing must expire the cookie with the same attributes: %+v", cleared)
	}
}

func TestTokenFromRequestPrefersTheCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := func(cookie, bearer string) *gin.Context {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: CookieName, Value: cookie})
		}
		if bearer != "" {
			r.Header.Set("Authorization", bearer)
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r
		return c
	}
	for _, tc := range []struct{ cookie, bearer, want string }{
		{"cookie-tok", "Bearer header-tok", "cookie-tok"},
		{"", "Bearer header-tok", "header-tok"},
		{"", "Basic abc", ""},
		{"", "Bearer", ""},
		{"", "", ""},
	} {
		if got := TokenFromRequest(req(tc.cookie, tc.bearer)); got != tc.want {
			t.Errorf("cookie=%q bearer=%q: got %q, want %q", tc.cookie, tc.bearer, got, tc.want)
		}
	}
}

// ---- sessions (database) ---------------------------------------------------------------------

func newUser(t *testing.T, db *gorm.DB, email string) models.User {
	t.Helper()
	u := models.User{Email: email, Password: "hash", HasPassword: true}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func TestSessionLifecycle(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "a@example.com")

	token, session, err := CreateSession(user.ID, strings.Repeat("x", 500))
	if err != nil {
		t.Fatal(err)
	}
	if len(session.UserAgent) != 200 {
		t.Errorf("user agent should be truncated to 200, got %d", len(session.UserAgent))
	}
	if time.Until(session.ExpiresAt) < 29*24*time.Hour {
		t.Errorf("sessions should last ~30 days: %v", time.Until(session.ExpiresAt))
	}

	s, u, err := LookupSession(token)
	if err != nil || s.ID != session.ID || u.ID != user.ID {
		t.Fatalf("lookup: %v %+v %+v", err, s, u)
	}
	for _, bad := range []string{"", "nope", token + "x", strings.ToUpper(token)} {
		if _, _, err := LookupSession(bad); err != ErrInvalidSession {
			t.Errorf("token %q: got %v, want ErrInvalidSession", bad, err)
		}
	}

	revoked, err := RevokeSession(user.ID, session.ID)
	if err != nil || !revoked {
		t.Fatalf("revoke: %v %v", revoked, err)
	}
	if _, _, err := LookupSession(token); err != ErrInvalidSession {
		t.Fatal("a revoked session must not authenticate")
	}
	if again, _ := RevokeSession(user.ID, session.ID); again {
		t.Error("revoking twice should report nothing revoked")
	}
}

func TestSessionsDieWithTheirUser(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "gone@example.com")
	token, _, _ := CreateSession(user.ID, "")

	db.Unscoped().Delete(&models.User{}, user.ID)
	if _, _, err := LookupSession(token); err != ErrInvalidSession {
		t.Fatalf("got %v", err)
	}
	var n int
	db.Raw("SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Errorf("sessions should cascade-delete with the user, %d remain", n)
	}
}

func TestRevokeAllKeepsOnlyTheNamedSession(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "multi@example.com")
	other := newUser(t, db, "other@example.com")
	t1, s1, _ := CreateSession(user.ID, "laptop")
	t2, _, _ := CreateSession(user.ID, "phone")
	t3, _, _ := CreateSession(other.ID, "someone else")

	if err := RevokeAllSessions(user.ID, s1.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LookupSession(t1); err != nil {
		t.Error("the kept session should survive")
	}
	if _, _, err := LookupSession(t2); err != ErrInvalidSession {
		t.Error("the other session should be revoked")
	}
	if _, _, err := LookupSession(t3); err != nil {
		t.Error("another user's session must be untouched")
	}
	list, _ := ActiveSessions(user.ID)
	if len(list) != 1 || list[0].ID != s1.ID {
		t.Errorf("active sessions: %+v", list)
	}
}

func TestSessionTTLCanBeShortenedAndExpires(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "ttl@example.com")
	old := SessionTTL
	SessionTTL = time.Minute
	t.Cleanup(func() { SessionTTL = old })

	token, _, _ := CreateSession(user.ID, "")
	db.Exec("UPDATE sessions SET expires_at = ?", time.Now().Add(-time.Second))
	if _, _, err := LookupSession(token); err != ErrInvalidSession {
		t.Fatal("an expired session must not authenticate")
	}
}

// ---- email tokens (database) -----------------------------------------------------------------

func TestEmailTokenIsSingleUseAndPurposeBound(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "tok@example.com")

	token, err := IssueEmailToken(user.ID, models.TokenResetPassword, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var stored int
	db.Raw("SELECT count(*) FROM email_tokens WHERE token_hash = ?", token).Scan(&stored)
	if stored != 0 {
		t.Fatal("only the hash may be stored")
	}

	if _, err := ConsumeEmailToken(token, models.TokenVerifyEmail); err != ErrInvalidToken {
		t.Error("a token must only work for its own purpose")
	}
	id, err := ConsumeEmailToken(token, models.TokenResetPassword)
	if err != nil || id != user.ID {
		t.Fatalf("consume: %v %v", id, err)
	}
	if _, err := ConsumeEmailToken(token, models.TokenResetPassword); err != ErrInvalidToken {
		t.Error("a token must be single-use")
	}
}

func TestEmailTokenExpiryAndReplacement(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "exp@example.com")

	short, _ := IssueEmailToken(user.ID, models.TokenVerifyEmail, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, err := ConsumeEmailToken(short, models.TokenVerifyEmail); err != ErrInvalidToken {
		t.Error("an expired token must be rejected")
	}

	first, _ := IssueEmailToken(user.ID, models.TokenVerifyEmail, time.Hour)
	second, _ := IssueEmailToken(user.ID, models.TokenVerifyEmail, time.Hour)
	if _, err := ConsumeEmailToken(first, models.TokenVerifyEmail); err != ErrInvalidToken {
		t.Error("issuing a new token must invalidate the previous one")
	}
	if _, err := ConsumeEmailToken(second, models.TokenVerifyEmail); err != nil {
		t.Errorf("the newest token should work: %v", err)
	}
	if _, err := ConsumeEmailToken("", models.TokenVerifyEmail); err != ErrInvalidToken {
		t.Error("an empty token is invalid")
	}
}

func TestEmailTokenCanOnlyBeRedeemedOnceUnderConcurrency(t *testing.T) {
	db := testutil.DB(t, "authtest")
	user := newUser(t, db, "race@example.com")
	token, _ := IssueEmailToken(user.ID, models.TokenResetPassword, time.Hour)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ConsumeEmailToken(token, models.TokenResetPassword); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("a reset link redeemed by 20 simultaneous requests must succeed exactly once, got %d", wins)
	}
	_ = initializers.DB
}
