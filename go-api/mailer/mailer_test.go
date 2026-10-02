package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func clearMailEnv(t *testing.T) {
	for _, k := range []string{"MAIL_PROVIDER", "MAIL_FROM", "BREVO_API_KEY", "RESEND_API_KEY"} {
		t.Setenv(k, "")
	}
}

func TestFromEnvDefaultsToTheLogProvider(t *testing.T) {
	clearMailEnv(t)
	m, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.(LogMailer); !ok {
		t.Fatalf("got %T, want LogMailer", m)
	}
	t.Setenv("MAIL_PROVIDER", " LOG ")
	if m, _ := FromEnv(); m == nil {
		t.Fatal("provider names should be case and space insensitive")
	}
}

func TestFromEnvRejectsMisconfiguration(t *testing.T) {
	cases := map[string]map[string]string{
		"unknown provider":     {"MAIL_PROVIDER": "mailgun"},
		"brevo without key":    {"MAIL_PROVIDER": "brevo", "MAIL_FROM": "Me <me@example.com>"},
		"brevo without sender": {"MAIL_PROVIDER": "brevo", "BREVO_API_KEY": "k"},
		"brevo bad sender":     {"MAIL_PROVIDER": "brevo", "BREVO_API_KEY": "k", "MAIL_FROM": "not an address"},
		"resend without key":   {"MAIL_PROVIDER": "resend", "MAIL_FROM": "me@example.com"},
		"resend without from":  {"MAIL_PROVIDER": "resend", "RESEND_API_KEY": "k"},
	}
	for name, env := range cases {
		clearMailEnv(t)
		for k, v := range env {
			t.Setenv(k, v)
		}
		if _, err := FromEnv(); err == nil {
			t.Errorf("%s should fail at startup", name)
		}
	}
}

func TestParseSender(t *testing.T) {
	name, addr, err := parseSender("AI Summarizer <noreply@example.com>")
	if err != nil || name != "AI Summarizer" || addr != "noreply@example.com" {
		t.Errorf("got %q %q %v", name, addr, err)
	}
	if name, addr, err := parseSender("bare@example.com"); err != nil || name != "" || addr != "bare@example.com" {
		t.Errorf("bare address: %q %q %v", name, addr, err)
	}
}

type captured struct {
	headers http.Header
	body    map[string]any
}

func fakeProvider(t *testing.T, status int, reply string) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.headers = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &c.body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func TestBrevoRequestShape(t *testing.T) {
	srv, got := fakeProvider(t, 201, `{"messageId":"x"}`)
	old := brevoURL
	brevoURL = srv.URL
	t.Cleanup(func() { brevoURL = old })

	m, err := newBrevo("secret-key", "AI Summarizer <noreply@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	err = m.Send(context.Background(), Message{To: "user@example.com", Subject: "Hi", Text: "plain", HTML: "<b>html</b>"})
	if err != nil {
		t.Fatal(err)
	}

	if got.headers.Get("api-key") != "secret-key" || got.headers.Get("Content-Type") != "application/json" {
		t.Errorf("headers: %v", got.headers)
	}
	sender := got.body["sender"].(map[string]any)
	to := got.body["to"].([]any)[0].(map[string]any)
	if sender["email"] != "noreply@example.com" || sender["name"] != "AI Summarizer" || to["email"] != "user@example.com" {
		t.Errorf("addresses: %v", got.body)
	}
	if got.body["subject"] != "Hi" || got.body["textContent"] != "plain" || got.body["htmlContent"] != "<b>html</b>" {
		t.Errorf("content: %v", got.body)
	}
}

func TestResendRequestShape(t *testing.T) {
	srv, got := fakeProvider(t, 200, `{"id":"x"}`)
	old := resendURL
	resendURL = srv.URL
	t.Cleanup(func() { resendURL = old })

	m, err := newResend("re_secret", "AI Summarizer <noreply@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Send(context.Background(), Message{To: "user@example.com", Subject: "Hi", Text: "plain", HTML: "<b>h</b>"}); err != nil {
		t.Fatal(err)
	}

	if got.headers.Get("Authorization") != "Bearer re_secret" {
		t.Errorf("auth header: %v", got.headers)
	}
	if got.body["from"] != "AI Summarizer <noreply@example.com>" || got.body["subject"] != "Hi" || got.body["text"] != "plain" || got.body["html"] != "<b>h</b>" {
		t.Errorf("body: %v", got.body)
	}
	if to := got.body["to"].([]any); len(to) != 1 || to[0] != "user@example.com" {
		t.Errorf("to: %v", got.body["to"])
	}
}

func TestProviderErrorsAreReportedWithoutTheKey(t *testing.T) {
	srv, _ := fakeProvider(t, 401, `{"message":"Invalid API key"}`)
	old := brevoURL
	brevoURL = srv.URL
	t.Cleanup(func() { brevoURL = old })

	m, _ := newBrevo("super-secret-key", "me@example.com")
	err := m.Send(context.Background(), Message{To: "u@example.com", Subject: "s"})
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid API key") {
		t.Fatalf("expected the provider's explanation, got %v", err)
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Fatal("the API key must never appear in errors")
	}
}

func TestNetworkFailuresAreErrors(t *testing.T) {
	old := resendURL
	resendURL = "http://127.0.0.1:1"
	t.Cleanup(func() { resendURL = old })
	m, _ := newResend("k", "me@example.com")
	if err := m.Send(context.Background(), Message{To: "u@example.com"}); err == nil {
		t.Fatal("expected an error")
	}
}

// ---- SendAsync -------------------------------------------------------------------------------

type recorder struct {
	mu   sync.Mutex
	sent []Message
	err  error
}

func (r *recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return r.err
}

func TestSendAsyncDoesNotBlockTheCaller(t *testing.T) {
	slow := make(chan struct{})
	m := mailerFunc(func(ctx context.Context, msg Message) error { <-slow; return nil })

	start := time.Now()
	SendAsync(m, Message{To: "u@example.com"})
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("SendAsync must return immediately so response time doesn't reveal whether an email was sent")
	}
	close(slow)
}

type mailerFunc func(context.Context, Message) error

func (f mailerFunc) Send(ctx context.Context, m Message) error { return f(ctx, m) }

func TestSendAsyncLogsFailuresWithoutTheRecipientAddress(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&lockedWriter{&buf, &mu}, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	done := make(chan struct{})
	SendAsync(mailerFunc(func(context.Context, Message) error { defer close(done); return io.ErrUnexpectedEOF }),
		Message{To: "private.person@example.com", Subject: "Reset"})
	<-done
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	out := buf.String()
	if !strings.Contains(out, "sending email failed") || !strings.Contains(out, "example.com") {
		t.Fatalf("failure not logged: %s", out)
	}
	if strings.Contains(out, "private.person") {
		t.Fatalf("the recipient's address must not be logged: %s", out)
	}
}

type lockedWriter struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// ---- templates -------------------------------------------------------------------------------

func TestTemplatesCarryTheLinkAndEscapeHTML(t *testing.T) {
	m := VerifyEmail("u@example.com", "https://app.example.com/", "tok+/=123")
	wantLink := "https://app.example.com/verify-email?token=tok%2B%2F%3D123"
	if !strings.Contains(m.Text, wantLink) || !strings.Contains(m.HTML, wantLink) {
		t.Errorf("link missing or unescaped:\n%s", m.Text)
	}
	if m.To != "u@example.com" || m.Subject == "" {
		t.Errorf("%+v", m)
	}

	r := ResetPassword("u@example.com", "http://localhost:3000", "abc")
	if !strings.Contains(r.Text, "http://localhost:3000/reset-password?token=abc") || !strings.Contains(r.Text, "1 hour") {
		t.Errorf("reset email: %s", r.Text)
	}

	// the frontend URL comes from our own config, but the HTML escaping must still hold
	evil := VerifyEmail("u@example.com", `https://x.example/"><script>alert(1)</script>`, "t")
	if strings.Contains(evil.HTML, "<script>") {
		t.Errorf("HTML not escaped: %s", evil.HTML)
	}
}

func TestPasswordChangedNoticeHasNoLink(t *testing.T) {
	m := PasswordChanged("u@example.com")
	if strings.Contains(m.Text, "http") || !strings.Contains(m.Text, "wasn't you") {
		t.Errorf("%+v", m)
	}
}
