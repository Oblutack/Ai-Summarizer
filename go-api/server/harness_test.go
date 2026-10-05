package server

// Shared test harness: the real router served over real HTTP, a Postgres schema of its own, a fake
// mail provider that records what would have been sent, and a fake AI service.

import (
	"ai-summarizer/go-api/auth"
	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/mailer"
	"ai-summarizer/go-api/models"
	"ai-summarizer/go-api/testutil"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	appOrigin = "http://app.test"
	goodPass  = "Correct-horse-9"
)

// ---- fake mail ---------------------------------------------------------------------------

type sentMail struct{ To, Subject, Text string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (f *fakeMailer) Send(_ context.Context, m mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{m.To, m.Subject, m.Text})
	return nil
}

func (f *fakeMailer) all() []sentMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMail(nil), f.sent...)
}

// waitFor blocks until at least n emails were sent (emails go out in the background).
func (f *fakeMailer) waitFor(t *testing.T, n int) []sentMail {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := f.all(); len(got) >= n {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d email(s), got %d: %+v", n, len(f.all()), f.all())
	return nil
}

var tokenRE = regexp.MustCompile(`token=([A-Za-z0-9_%\-]+)`)

// tokenIn extracts the token from the link in an email.
func tokenIn(t *testing.T, m sentMail) string {
	t.Helper()
	match := tokenRE.FindStringSubmatch(m.Text)
	if match == nil {
		t.Fatalf("no token link in email %q:\n%s", m.Subject, m.Text)
	}
	tok, err := url.QueryUnescape(match[1])
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// ---- fake AI service ---------------------------------------------------------------------

type fakeAI struct {
	calls atomic.Int32
	fail  atomic.Bool
	// streamError makes streamed summaries end with an error event.
	streamError atomic.Bool
}

func (f *fakeAI) handler(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	if f.fail.Load() {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"internal traceback"}`))
		return
	}
	if r.URL.Query().Get("stream") == "true" {
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{`{"type":"status","stage":"preparing"}`, `{"type":"delta","text":"fake "}`}
		if f.streamError.Load() {
			events = append(events, `{"type":"error","status":502,"message":"The language model failed to produce a summary. Please try again."}`)
		} else {
			events = append(events, `{"type":"delta","text":"summary"}`, `{"type":"done","text":"the source text"}`)
		}
		for _, e := range events {
			_, _ = w.Write([]byte("data: " + e + "\n\n"))
			w.(http.Flusher).Flush()
		}
		return
	}
	switch r.URL.Path {
	case "/chat":
		_, _ = w.Write([]byte(`{"answer":"fake answer"}`))
	case "/healthz":
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	default:
		_, _ = w.Write([]byte(`{"summary":"fake summary"}`))
	}
}

// ---- the app ------------------------------------------------------------------------------

type app struct {
	t    *testing.T
	srv  *httptest.Server
	db   *gorm.DB
	ai   *fakeAI
	mail *fakeMailer
}

var generousRates = Rates{
	AuthPerMinute: 1000, AuthBurst: 1000,
	SummarizeIPPerMinute: 1000, SummarizeIPBurst: 1000,
	SummarizeUserPerMinute: 1000, SummarizeUserBurst: 1000,
	ChatPerMinute: 1000, ChatBurst: 1000,
	ExportPerMinute: 1000, ExportBurst: 1000,
}

func newApp(t *testing.T) *app { return newAppWithRates(t, generousRates) }

func newAppWithRates(t *testing.T, rates Rates) *app {
	t.Helper()
	db := testutil.DB(t, "srvtest")

	t.Setenv("CORS_ALLOWED_ORIGINS", appOrigin)
	t.Setenv("FRONTEND_URL", appOrigin)
	for _, k := range []string{"TURNSTILE_SECRET", "REQUIRE_EMAIL_VERIFICATION", "QUOTA_SUMMARIES_PER_DAY", "QUOTA_CHAT_PER_DAY"} {
		t.Setenv(k, "")
	}

	ai := &fakeAI{}
	aiSrv := httptest.NewServer(http.HandlerFunc(ai.handler))
	t.Cleanup(aiSrv.Close)
	t.Setenv("AI_SERVICE_URL", aiSrv.URL)

	oldCookie, oldMail := auth.Cookie, controllers.Mail
	auth.Cookie = auth.CookieConfig{Secure: false, SameSite: http.SameSiteLaxMode}
	mail := &fakeMailer{}
	controllers.Mail = mail
	t.Cleanup(func() { auth.Cookie, controllers.Mail = oldCookie, oldMail })

	gin.SetMode(gin.TestMode)
	router, err := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), rates)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &app{t: t, srv: srv, db: db, ai: ai, mail: mail}
}

// ---- a browser-like client ---------------------------------------------------------------

type client struct {
	a      *app
	http   *http.Client
	origin string // sent as the Origin header on state-changing requests, like a browser does
}

func (a *app) newClient() *client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		a.t.Fatal(err)
	}
	return &client{
		a:      a,
		origin: appOrigin,
		http: &http.Client{
			Jar:           jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

type reply struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r reply) JSON() map[string]any {
	var out map[string]any
	_ = json.Unmarshal(r.Body, &out)
	return out
}

func (r reply) Str(key string) string {
	s, _ := r.JSON()[key].(string)
	return s
}

func (r reply) cookies() []*http.Cookie { return (&http.Response{Header: r.Header}).Cookies() }

func (r reply) cookie(name string) *http.Cookie {
	for _, c := range r.cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// req performs a request. body may be nil, a string (sent as is) or anything JSON-encodable;
// headers are key, value pairs.
func (cl *client) req(method, path string, body any, headers ...string) reply {
	cl.a.t.Helper()
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case string:
		reader, contentType = bytes.NewBufferString(b), "application/json"
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			cl.a.t.Fatal(err)
		}
		reader, contentType = bytes.NewReader(raw), "application/json"
	}

	req, err := http.NewRequest(method, cl.a.srv.URL+path, reader)
	if err != nil {
		cl.a.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cl.origin != "" && method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", cl.origin)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}

	resp, err := cl.http.Do(req)
	if err != nil {
		cl.a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return reply{resp.StatusCode, resp.Header, data}
}

func (cl *client) get(path string) reply              { return cl.req(http.MethodGet, path, nil) }
func (cl *client) post(path string, body any) reply   { return cl.req(http.MethodPost, path, body) }
func (cl *client) delete(path string, body any) reply { return cl.req(http.MethodDelete, path, body) }

// sessionToken returns the raw session token the client currently holds.
func (cl *client) sessionToken() string {
	u, _ := url.Parse(cl.a.srv.URL)
	for _, c := range cl.http.Jar.Cookies(u) {
		if c.Name == auth.CookieName {
			return c.Value
		}
	}
	return ""
}

// ---- account helpers ----------------------------------------------------------------------

var emailCounter atomic.Int32

func uniqueEmail() string {
	return "user" + time.Now().Format("150405") + "-" + string('a'+emailCounter.Add(1)%26) + string('a'+(emailCounter.Load()/26)%26) + "@example.com"
}

// signup creates an account (unverified) and returns its email.
func (a *app) signup(email string) {
	a.t.Helper()
	cl := a.newClient()
	if r := cl.post("/signup", map[string]string{"email": email, "password": goodPass}); r.Status != http.StatusOK {
		a.t.Fatalf("signup %s: %d %s", email, r.Status, r.Body)
	}
}

// login signs the client in and fails the test otherwise.
func (a *app) login(cl *client, email, password string) {
	a.t.Helper()
	if r := cl.post("/login", map[string]string{"email": email, "password": password}); r.Status != http.StatusOK {
		a.t.Fatalf("login %s: %d %s", email, r.Status, r.Body)
	}
}

// newUser signs up and logs in, returning a signed-in client and the account's email.
func (a *app) newUser() (*client, string) {
	a.t.Helper()
	email := uniqueEmail()
	a.signup(email)
	cl := a.newClient()
	a.login(cl, email, goodPass)
	return cl, email
}

func (a *app) userByEmail(email string) models.User {
	a.t.Helper()
	var u models.User
	if err := a.db.Unscoped().First(&u, "email = ?", email).Error; err != nil {
		a.t.Fatalf("user %s: %v", email, err)
	}
	return u
}

func count(t *testing.T, db *gorm.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

var itoa = strconv.Itoa
