package server

import (
	"net/http"
	"strings"
	"sync"
	"testing"
)

func summarize(cl *client) reply {
	return cl.post("/summarize-text", map[string]string{"text": "Heat pumps are efficient. Sales rose in 2024."})
}

// ---- quotas --------------------------------------------------------------------------------

func TestDailySummaryQuotaIsEnforced(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()

	for i := 1; i <= 3; i++ {
		r := summarize(cl)
		if r.Status != 200 {
			t.Fatalf("request %d: %d %s", i, r.Status, r.Body)
		}
		if r.Header.Get("X-Quota-Limit") != "3" || r.Header.Get("X-Quota-Remaining") != itoa(3-i) {
			t.Errorf("request %d: quota headers %q/%q", i, r.Header.Get("X-Quota-Limit"), r.Header.Get("X-Quota-Remaining"))
		}
	}

	r := summarize(cl)
	if r.Status != http.StatusTooManyRequests || r.Str("code") != "quota_exceeded" || r.Header.Get("X-Quota-Remaining") != "0" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("4th request: %d %s %v", r.Status, r.Body, r.Header)
	}
	if !strings.Contains(r.Str("error"), "midnight UTC") {
		t.Errorf("the message should say when it resets: %s", r.Str("error"))
	}
	if a.ai.calls.Load() != 3 {
		t.Errorf("the AI must not be called once the quota is spent: %d calls", a.ai.calls.Load())
	}
}

func TestQuotasArePerUserAndPerKind(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "1")
	t.Setenv("QUOTA_CHAT_PER_DAY", "1")
	alice, _ := a.newUser()
	bob, _ := a.newUser()

	summarize(alice)
	if summarize(alice).Status != http.StatusTooManyRequests {
		t.Fatal("Alice's summaries are spent")
	}
	if summarize(bob).Status != 200 {
		t.Error("Bob has his own allowance")
	}

	// chat is a separate allowance: Alice's spent summaries don't stop her chatting
	a.db.Exec("INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content) SELECT now(), now(), 'd', 's', id, true, 'text' FROM users")
	var docID uint
	a.db.Raw("SELECT d.id FROM documents d JOIN users u ON u.id = d.user_id WHERE u.id = (SELECT user_id FROM sessions ORDER BY id LIMIT 1)").Scan(&docID)
	if r := alice.post("/documents/"+itoa(int(docID))+"/chat", map[string]string{"question": "hi"}); r.Status != 200 {
		t.Errorf("chat after summaries are spent: %d %s", r.Status, r.Body)
	}
	if r := alice.post("/documents/"+itoa(int(docID))+"/chat", map[string]string{"question": "again"}); r.Status != http.StatusTooManyRequests || r.Str("code") != "quota_exceeded" {
		t.Errorf("second chat should hit the chat quota: %d %s", r.Status, r.Body)
	}
}

func TestQuotaIsAtomicUnderConcurrency(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()

	var mu sync.Mutex
	ok := 0
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if summarize(cl).Status == 200 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 3 {
		t.Fatalf("exactly 3 of 12 simultaneous requests should succeed, got %d", ok)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 3 {
		t.Errorf("recorded usage = %d, want 3", n)
	}
}

func TestFailedWorkIsRefunded(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "2")
	cl, _ := a.newUser()

	a.ai.fail.Store(true)
	for i := 0; i < 5; i++ {
		if r := summarize(cl); r.Status != http.StatusBadGateway {
			t.Fatalf("AI failure should be a 502, got %d", r.Status)
		}
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 0 {
		t.Errorf("failed requests must not use up the allowance, usage = %d", n)
	}

	a.ai.fail.Store(false)
	if summarize(cl).Status != 200 || summarize(cl).Status != 200 {
		t.Error("the full allowance should still be available after the outage")
	}
}

func TestInvalidRequestsAreNotCharged(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "2")
	cl, _ := a.newUser()

	cl.post("/summarize-text", map[string]string{"text": "   "})
	cl.post("/summarize-text?style=poem", map[string]string{"text": "hello"})
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 0 {
		t.Errorf("rejected requests were charged: usage = %d", n)
	}
}

func TestStreamedFailuresAreRefundedButSuccessesAreCharged(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "5")
	cl, _ := a.newUser()

	a.ai.streamError.Store(true)
	r := cl.post("/summarize-text?stream=true", map[string]string{"text": "hello world"})
	if !strings.Contains(string(r.Body), `"type":"error"`) {
		t.Fatalf("expected an error event: %s", r.Body)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 0 {
		t.Errorf("a stream that ended in an error must be refunded, usage = %d", n)
	}

	a.ai.streamError.Store(false)
	r = cl.post("/summarize-text?stream=true", map[string]string{"text": "hello world"})
	if !strings.Contains(string(r.Body), `"type":"done"`) {
		t.Fatalf("expected a completed stream: %s", r.Body)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 1 {
		t.Errorf("a completed stream costs one use, usage = %d", n)
	}
}

func TestQuotaCanBeDisabled(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "0")
	cl, _ := a.newUser()
	for i := 0; i < 5; i++ {
		if r := summarize(cl); r.Status != 200 {
			t.Fatalf("request %d: %d", i, r.Status)
		}
	}
	if r := summarize(cl); r.Header.Get("X-Quota-Limit") != "" {
		t.Error("an unlimited quota shouldn't advertise a limit")
	}
}

func TestUsageEndpoint(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "10")
	t.Setenv("QUOTA_CHAT_PER_DAY", "40")
	cl, _ := a.newUser()
	summarize(cl)
	summarize(cl)

	r := cl.get("/usage")
	summaries, _ := r.JSON()["summaries"].(map[string]any)
	chats, _ := r.JSON()["chats"].(map[string]any)
	if r.Status != 200 || summaries["used"] != float64(2) || summaries["limit"] != float64(10) || chats["used"] != float64(0) || chats["limit"] != float64(40) {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if r.Str("resetsAt") == "" {
		t.Error("missing resetsAt")
	}
	if a.newClient().get("/usage").Status != 401 {
		t.Error("usage requires a session")
	}
}

func TestAnonymousSummariesAreNotChargedToAnyone(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "1")
	cl := a.newClient()
	for i := 0; i < 3; i++ {
		if r := cl.post("/public/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
			t.Fatalf("anonymous request %d: %d", i, r.Status)
		}
	}
}

// ---- export and deletion ---------------------------------------------------------------------

func seedDocs(a *app, email string, n int) {
	a.db.Exec(`INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content)
		SELECT now(), now(), 'doc-' || g, 'summary ' || g, ?, true, 'SOURCE-' || g FROM generate_series(1, ?) g`,
		a.userByEmail(email).ID, n)
}

func TestExportContainsOnlyTheUsersOwnDataIncludingSourceText(t *testing.T) {
	a := newApp(t)
	alice, aliceEmail := a.newUser()
	_, bobEmail := a.newUser()
	seedDocs(a, aliceEmail, 2)
	seedDocs(a, bobEmail, 3)
	summarize(alice)

	r := alice.get("/account/export")
	if r.Status != 200 || !strings.Contains(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("%d %v", r.Status, r.Header)
	}
	body := string(r.Body)
	if !strings.Contains(body, "SOURCE-1") || !strings.Contains(body, "summary 2") || !strings.Contains(body, aliceEmail) {
		t.Errorf("Alice's data should be exported: %s", body)
	}
	if strings.Contains(body, bobEmail) || strings.Contains(body, "doc-3") {
		t.Error("another user's data leaked into the export")
	}
	if strings.Contains(body, "password") || strings.Contains(body, "$2a$") {
		t.Error("the password hash must never be exported")
	}
	docs, _ := r.JSON()["documents"].([]any)
	if len(docs) != 3 { // two seeded + one created by summarize()
		t.Errorf("expected 3 documents, got %d", len(docs))
	}
	if a.newClient().get("/account/export").Status != 401 {
		t.Error("export requires a session")
	}
}

func TestDeleteAccountRemovesEverythingForReal(t *testing.T) {
	a := newApp(t)
	alice, aliceEmail := a.newUser()
	bob, bobEmail := a.newUser()
	seedDocs(a, aliceEmail, 3)
	seedDocs(a, bobEmail, 2)
	summarize(alice)
	a.db.Exec("UPDATE documents SET deleted_at = now() WHERE filename = 'doc-1' AND user_id = ?", a.userByEmail(aliceEmail).ID) // soft-deleted rows must go too
	aliceID := a.userByEmail(aliceEmail).ID

	// confirmation is required
	if r := alice.delete("/account", map[string]string{"confirmEmail": "someone.else@example.com", "password": goodPass}); r.Status != http.StatusBadRequest {
		t.Errorf("wrong confirmation email: %d", r.Status)
	}
	if r := alice.delete("/account", map[string]string{"confirmEmail": aliceEmail, "password": "Wrong-password-1"}); r.Status != http.StatusForbidden {
		t.Errorf("wrong password must be 403, not 401: %d", r.Status)
	}
	if alice.get("/auth/me").Status != 200 {
		t.Fatal("failed attempts must not delete or log out anything")
	}

	r := alice.delete("/account", map[string]string{"confirmEmail": strings.ToUpper(aliceEmail), "password": goodPass})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if c := r.cookie("session"); c == nil || c.MaxAge >= 0 {
		t.Error("the session cookie should be cleared")
	}

	for table, query := range map[string]string{
		"users":        "SELECT count(*) FROM users WHERE id = ?",
		"documents":    "SELECT count(*) FROM documents WHERE user_id = ?",
		"sessions":     "SELECT count(*) FROM sessions WHERE user_id = ?",
		"email_tokens": "SELECT count(*) FROM email_tokens WHERE user_id = ?",
		"daily_usage":  "SELECT count(*) FROM daily_usage WHERE user_id = ?",
	} {
		if n := count(t, a.db, query, aliceID); n != 0 {
			t.Errorf("%s: %d rows remain for the deleted user (hard delete expected)", table, n)
		}
	}
	if a.newClient().post("/login", map[string]string{"email": aliceEmail, "password": goodPass}).Status != 401 {
		t.Error("the deleted account must not be able to log in")
	}
	if alice.get("/auth/me").Status != 401 {
		t.Error("the deleted account's session must be dead")
	}

	// Bob is untouched
	if bob.get("/auth/me").Status != 200 || count(t, a.db, "SELECT count(*) FROM documents WHERE user_id = ?", a.userByEmail(bobEmail).ID) != 2 {
		t.Error("another user's data was affected")
	}
}

func TestGoogleOnlyAccountsConfirmDeletionByEmailAlone(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	a.db.Exec("UPDATE users SET has_password = false WHERE email = ?", email)

	if r := cl.delete("/account", map[string]string{"confirmEmail": "wrong@example.com"}); r.Status != http.StatusBadRequest {
		t.Errorf("wrong email: %d", r.Status)
	}
	if r := cl.delete("/account", map[string]string{"confirmEmail": email}); r.Status != 200 {
		t.Errorf("an account with no password confirms by email: %d %s", r.Status, r.Body)
	}
}

func TestDeleteAccountRequiresASession(t *testing.T) {
	a := newApp(t)
	if r := a.newClient().delete("/account", map[string]string{"confirmEmail": "a@b.co"}); r.Status != http.StatusUnauthorized {
		t.Errorf("got %d", r.Status)
	}
}
