package server

import (
	"net/http"
	"strings"
	"sync"
	"testing"
)

// The spending guard: how much AI work one visitor without an account, and the whole site, can start in a day.
// (The harness switches both limits off; these tests turn them on.)

func anonymousSummary(cl *client, forwardedFor ...string) reply {
	var headers []string
	for _, ip := range forwardedFor {
		headers = append(headers, "X-Forwarded-For", ip)
	}
	return cl.req(http.MethodPost, "/public/summarize-text", map[string]string{"text": "Heat pumps are efficient. Sales rose in 2024."}, headers...)
}

func usedToday(t *testing.T, a *app, table, column string) int {
	t.Helper()
	return count(t, a.db, "SELECT COALESCE(SUM("+column+"), 0) FROM "+table+" WHERE day = CURRENT_DATE")
}

// ---- visitors without an account ----------------------------------------------------------------

func TestAVisitorWithoutAnAccountHasADailyAllowanceOfSummaries(t *testing.T) {
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "2")
	anon := a.newClient()

	for i := 1; i <= 2; i++ {
		r := anonymousSummary(anon)
		if r.Status != http.StatusOK {
			t.Fatalf("summary %d: %d %s", i, r.Status, r.Body)
		}
		if got, want := r.Header.Get("X-Anonymous-Remaining"), itoa(2-i); got != want {
			t.Errorf("summary %d: remaining %q, want %q", i, got, want)
		}
	}
	r := anonymousSummary(anon)
	if r.Status != http.StatusTooManyRequests || r.Str("code") != "anonymous_limit" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("3rd summary: %d %s %v", r.Status, r.Body, r.Header)
	}
	if msg := r.Str("error"); !strings.Contains(msg, "Sign in") || !strings.Contains(msg, "midnight UTC") || !strings.Contains(msg, "2 free summaries") {
		t.Errorf("the message should say what to do and when it resets: %s", msg)
	}
	if n := a.ai.calls.Load(); n != 2 {
		t.Errorf("the AI must not be called once the allowance is spent: %d calls", n)
	}

	// every anonymous route shares the allowance
	for _, path := range []string{"/public/summarize-url", "/public/summarize"} {
		if r := anon.post(path, map[string]string{"url": "https://news.example/post"}); r.Status != http.StatusTooManyRequests {
			t.Errorf("%s after the allowance: %d", path, r.Status)
		}
	}
	if r := anon.upload("/public/summarize-multiple", "files", map[string][]byte{"a.pdf": pdfBytes}); r.Status != http.StatusTooManyRequests {
		t.Errorf("/public/summarize-multiple after the allowance: %d", r.Status)
	}

	// signing in gives more, and nothing else is affected
	cl, _ := a.newUser()
	if r := summarize(cl); r.Status != http.StatusOK {
		t.Errorf("a signed-in user is not held back by the visitor limit: %d %s", r.Status, r.Body)
	}
	if r := anon.get("/options"); r.Status != http.StatusOK {
		t.Errorf("pages that need no AI still work: %d", r.Status)
	}
}

func TestVisitorsAreToldApartByAddressAndEachHasAnAllowance(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.0/8,::1/128") // so the address sent along by the proxy is believed
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "1")
	anon := a.newClient()

	if r := anonymousSummary(anon, "203.0.113.1"); r.Status != http.StatusOK {
		t.Fatalf("first visitor: %d %s", r.Status, r.Body)
	}
	if r := anonymousSummary(anon, "203.0.113.1"); r.Status != http.StatusTooManyRequests {
		t.Errorf("the same visitor again: %d", r.Status)
	}
	if r := anonymousSummary(anon, "203.0.113.2"); r.Status != http.StatusOK {
		t.Errorf("another visitor has their own allowance: %d %s", r.Status, r.Body)
	}

	// one IPv6 connection holds billions of addresses: the whole /64 is one visitor
	if r := anonymousSummary(anon, "2001:db8:1:2::1"); r.Status != http.StatusOK {
		t.Fatalf("an IPv6 visitor: %d %s", r.Status, r.Body)
	}
	if r := anonymousSummary(anon, "2001:db8:1:2:aaaa:bbbb:cccc:dddd"); r.Status != http.StatusTooManyRequests {
		t.Errorf("another address in the same /64 is the same visitor: %d", r.Status)
	}
	if r := anonymousSummary(anon, "2001:db8:1:3::1"); r.Status != http.StatusOK {
		t.Errorf("a different /64 is a different visitor: %d %s", r.Status, r.Body)
	}
}

func TestNoAddressIsKept(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.0/8,::1/128")
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "5")
	anonymousSummary(a.newClient(), "203.0.113.77")

	var hash string
	a.db.Raw("SELECT ip_hash FROM anonymous_usage LIMIT 1").Scan(&hash)
	if len(hash) != 32 || strings.Contains(hash, "203") || strings.Contains(hash, "113") {
		t.Errorf("only a keyed hash may be stored, found %q", hash)
	}
}

func TestAFailedOrRefusedAnonymousSummaryDoesNotUseTheAllowance(t *testing.T) {
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "1")
	anon := a.newClient()

	a.ai.fail.Store(true)
	if r := anonymousSummary(anon); r.Status != http.StatusBadGateway {
		t.Fatalf("a failing AI service: %d %s", r.Status, r.Body)
	}
	a.ai.fail.Store(false)
	if r := anon.post("/public/summarize-text", map[string]string{"text": "  "}); r.Status != http.StatusBadRequest {
		t.Fatalf("empty text: %d %s", r.Status, r.Body)
	}
	if n := usedToday(t, a, "anonymous_usage", "summaries"); n != 0 {
		t.Errorf("nothing was summarized, yet %d were counted", n)
	}
	if r := anonymousSummary(anon); r.Status != http.StatusOK {
		t.Errorf("the allowance is still there: %d %s", r.Status, r.Body)
	}
}

func TestTheVisitorLimitCanBeSwitchedOff(t *testing.T) {
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "0")
	anon := a.newClient()
	for i := 0; i < 15; i++ {
		if r := anonymousSummary(anon); r.Status != http.StatusOK {
			t.Fatalf("summary %d: %d %s", i, r.Status, r.Body)
		}
	}
}

// ---- the whole site ------------------------------------------------------------------------------

func TestTheSitePausesAIWorkWhenTheDailyBudgetIsUsedUp(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "3")
	cl, _ := a.newUser()
	a.db.Exec("INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content) SELECT now(), now(), 'd', 's', id, true, 'text' FROM users")
	var docID uint
	a.db.Raw("SELECT id FROM documents LIMIT 1").Scan(&docID)

	if r := summarize(cl); r.Status != http.StatusOK {
		t.Fatalf("1st: %d %s", r.Status, r.Body)
	}
	if r := summarize(cl); r.Status != http.StatusOK {
		t.Fatalf("2nd: %d %s", r.Status, r.Body)
	}
	if r := cl.post("/documents/"+itoa(int(docID))+"/chat", map[string]string{"question": "hi"}); r.Status != http.StatusOK {
		t.Fatalf("a chat counts too: %d %s", r.Status, r.Body)
	}

	r := summarize(cl)
	if r.Status != http.StatusServiceUnavailable || r.Str("code") != "daily_budget" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("4th: %d %s %v", r.Status, r.Body, r.Header)
	}
	if msg := r.Str("error"); !strings.Contains(msg, "midnight UTC") {
		t.Errorf("the message should say when it is back: %s", msg)
	}
	// everyone is held, with or without an account, on every route that starts AI work
	if r := anonymousSummary(a.newClient()); r.Status != http.StatusServiceUnavailable || r.Str("code") != "daily_budget" {
		t.Errorf("a visitor: %d %s", r.Status, r.Body)
	}
	other, _ := a.newUser()
	if r := summarize(other); r.Status != http.StatusServiceUnavailable {
		t.Errorf("another user: %d", r.Status)
	}
	if r := cl.post("/documents/"+itoa(int(docID))+"/chat", map[string]string{"question": "again"}); r.Status != http.StatusServiceUnavailable {
		t.Errorf("chat: %d", r.Status)
	}
	if r := cl.post("/library/ask", map[string]string{"question": "what?"}); r.Status != http.StatusServiceUnavailable {
		t.Errorf("ask the library: %d", r.Status)
	}
	if n := a.ai.calls.Load(); n != 3 {
		t.Errorf("no AI work may start after the budget: %d calls", n)
	}

	// what costs nothing keeps working
	for _, path := range []string{"/documents", "/usage", "/options", "/auth/me"} {
		if r := cl.get(path); r.Status != http.StatusOK {
			t.Errorf("%s while paused: %d %s", path, r.Status, r.Body)
		}
	}
}

func TestAnUnlimitedBudgetNeverPauses(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "0")
	cl, _ := a.newUser()
	for i := 0; i < 12; i++ {
		if r := summarize(cl); r.Status != http.StatusOK {
			t.Fatalf("summary %d: %d %s", i, r.Status, r.Body)
		}
	}
}

func TestOnlyWorkThatReallyStartsUsesTheBudget(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "10")
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "1")
	cl, _ := a.newUser()

	if r := summarize(cl); r.Status != http.StatusOK {
		t.Fatalf("1st: %d", r.Status)
	}
	if r := summarize(cl); r.Status != http.StatusTooManyRequests {
		t.Fatalf("the user's own quota: %d", r.Status)
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("a request refused by the user's quota used the site's budget: %d", n)
	}

	// failures give it back
	other, _ := a.newUser()
	a.ai.fail.Store(true)
	if r := summarize(other); r.Status != http.StatusBadGateway {
		t.Fatalf("a failing AI service: %d %s", r.Status, r.Body)
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("a failed summary still used the budget: %d", n)
	}
	// and so do mistakes in the request
	a.ai.fail.Store(false)
	if r := other.post("/summarize-text", map[string]string{"text": " "}); r.Status != http.StatusBadRequest {
		t.Fatalf("empty text: %d", r.Status)
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("a refused request used the budget: %d", n)
	}
}

func TestSuggestedQuestionsOnlyUseTheBudgetWhenTheyAreReallyMade(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "10")
	cl, _ := a.newUser()
	a.db.Exec("INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content) SELECT now(), now(), 'd', 'A summary.', id, true, 'text' FROM users")
	var docID uint
	a.db.Raw("SELECT id FROM documents LIMIT 1").Scan(&docID)
	path := "/documents/" + itoa(int(docID)) + "/suggestions"

	if r := cl.post(path, nil); r.Status != http.StatusOK {
		t.Fatalf("first time: %d %s", r.Status, r.Body)
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("making the questions is one piece of AI work: %d", n)
	}
	for i := 0; i < 3; i++ {
		if r := cl.post(path, nil); r.Status != http.StatusOK {
			t.Fatalf("again: %d %s", r.Status, r.Body)
		}
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("stored questions are free: %d used", n)
	}
	if r := cl.post("/documents/999999/suggestions", nil); r.Status != http.StatusNotFound {
		t.Errorf("an unknown document: %d", r.Status)
	}
	if n := usedToday(t, a, "global_usage", "requests"); n != 1 {
		t.Errorf("a missing document used the budget: %d", n)
	}
}

func TestTheBudgetCannotBeSqueezedPastByRequestsArrivingTogether(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "5")

	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := anonymousSummary(a.newClient())
			mu.Lock()
			statuses[r.Status]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[http.StatusOK] != 5 || statuses[http.StatusServiceUnavailable] != 20 {
		t.Errorf("exactly 5 may start: %v", statuses)
	}
	if n := a.ai.calls.Load(); n != 5 {
		t.Errorf("AI calls = %d, want 5", n)
	}
}

// ---- who is believed about their address -------------------------------------------------------------

func TestWithoutTrustedProxiesNobodyCanChooseTheirOwnAddress(t *testing.T) {
	a := newApp(t) // TRUSTED_PROXIES is not set
	t.Setenv("ANON_SUMMARIES_PER_DAY", "2")
	anon := a.newClient()

	for i, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		if r := anonymousSummary(anon, ip); r.Status != http.StatusOK {
			t.Fatalf("summary %d: %d %s", i+1, r.Status, r.Body)
		}
	}
	// pretending to be somebody new gets a visitor nothing: the connection is what counts
	for _, ip := range []string{"203.0.113.3", "198.51.100.9", "2001:db8::1"} {
		if r := anonymousSummary(anon, ip); r.Status != http.StatusTooManyRequests {
			t.Errorf("a made-up address %s was believed: %d", ip, r.Status)
		}
	}
}

func TestPretendingToBeAnotherAddressDoesNotDodgeTheRateLimit(t *testing.T) {
	rates := generousRates
	rates.SummarizeIPBurst, rates.SummarizeIPPerMinute = 2, 1
	a := newAppWithRates(t, rates)
	anon := a.newClient()

	got := map[int]int{}
	for i := 0; i < 6; i++ {
		got[anonymousSummary(anon, "203.0.113."+itoa(i+1)).Status]++
	}
	if got[200] != 2 || got[429] != 4 {
		t.Errorf("got %v", got)
	}
}

func TestOnlyTheListedProxiesAreBelieved(t *testing.T) {
	// the test's own connection comes from 127.0.0.1, which is not in this list
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")
	a := newApp(t)
	t.Setenv("ANON_SUMMARIES_PER_DAY", "1")
	anon := a.newClient()
	if r := anonymousSummary(anon, "203.0.113.1"); r.Status != http.StatusOK {
		t.Fatalf("first: %d", r.Status)
	}
	if r := anonymousSummary(anon, "203.0.113.2"); r.Status != http.StatusTooManyRequests {
		t.Errorf("a header from an address that is not a trusted proxy was believed: %d", r.Status)
	}
}
