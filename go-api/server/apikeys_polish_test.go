package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// More control over API keys: expiry, a daily limit of the key's own, counters, saving to the library, and the
// compare and extract routes.

func makeKeyWith(t *testing.T, cl *client, body map[string]any) (id uint, key string, r reply) {
	t.Helper()
	r = cl.post("/account/api-keys", body)
	if r.Status != http.StatusCreated {
		return 0, "", r
	}
	return uint(r.JSON()["id"].(float64)), r.Str("key"), r
}

func keyRow(t *testing.T, cl *client, id uint) map[string]any {
	t.Helper()
	var list []map[string]any
	if err := json.Unmarshal(cl.get("/account/api-keys").Body, &list); err != nil {
		t.Fatal(err)
	}
	for _, k := range list {
		if uint(k["id"].(float64)) == id {
			return k
		}
	}
	t.Fatalf("key %d is not listed", id)
	return nil
}

// ---- expiry -----------------------------------------------------------------------------------------

func TestAKeyCanBeMadeToExpire(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id, key, r := makeKeyWith(t, cl, map[string]any{"name": "short", "expiresInDays": 30})
	if r.Status != http.StatusCreated || r.JSON()["expiresAt"] == nil {
		t.Fatalf("create: %d %s", r.Status, r.Body)
	}
	if keyRow(t, cl, id)["expiresAt"] == nil {
		t.Error("the list shows when it expires")
	}
	if r := a.call(http.MethodGet, "/v1/usage", key, nil); r.Status != http.StatusOK {
		t.Fatalf("before it runs out: %d", r.Status)
	}

	a.db.Exec("UPDATE api_keys SET expires_at = now() - interval '1 minute' WHERE id = ?", id)
	r = a.call(http.MethodGet, "/v1/usage", key, nil)
	if r.Status != http.StatusUnauthorized || r.Str("code") != "api_key_expired" || !strings.Contains(r.Str("error"), "expired") {
		t.Errorf("after it ran out: %d %s", r.Status, r.Body)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusUnauthorized {
		t.Errorf("an expired key does no work: %d", r.Status)
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("no AI work for an expired key (%d calls)", n)
	}
}

func TestAKeyWithoutAnExpiryNeverExpiresAndBadExpiriesAreRefused(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	for _, body := range []map[string]any{{"name": "a"}, {"name": "b", "expiresInDays": 0}, {"name": "c", "expiresInDays": nil}} {
		id, _, r := makeKeyWith(t, cl, body)
		if r.Status != http.StatusCreated || keyRow(t, cl, id)["expiresAt"] != nil {
			t.Errorf("%v: %d %s", body, r.Status, r.Body)
		}
	}
	for _, days := range []any{-1, 367, 100000, "soon", 1.5} {
		if _, _, r := makeKeyWith(t, cl, map[string]any{"expiresInDays": days}); r.Status != http.StatusBadRequest {
			t.Errorf("expiresInDays %v: %d", days, r.Status)
		}
	}
	if _, _, r := makeKeyWith(t, cl, map[string]any{"expiresInDays": 366}); r.Status != http.StatusCreated {
		t.Errorf("a year is allowed: %d %s", r.Status, r.Body)
	}
}

// ---- a limit of the key's own and counters --------------------------------------------------------------

func TestAKeyCanHaveADailyLimitOfItsOwn(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"name": "capped", "dailyLimit": 2})
	other, otherKey := makeKey(t, cl, "free")

	for i := 1; i <= 2; i++ {
		if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusOK {
			t.Fatalf("request %d: %d %s", i, r.Status, r.Body)
		}
	}
	r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	if r.Status != http.StatusTooManyRequests || r.Str("code") != "api_key_limit" || r.Header.Get("Retry-After") == "" ||
		!strings.Contains(r.Str("error"), "daily limit of 2") {
		t.Fatalf("3rd request: %d %s %v", r.Status, r.Body, r.Header)
	}
	if n := a.ai.calls.Load(); n != 2 {
		t.Errorf("the AI must not be asked once the key is out (%d calls)", n)
	}
	// the refusal cost the owner nothing, and the other key of the same owner carries on
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage"); n != 2 {
		t.Errorf("the owner's allowance used: %d, want 2", n)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text", otherKey, textBody()); r.Status != http.StatusOK {
		t.Errorf("another key: %d %s", r.Status, r.Body)
	}
	_ = other

	row := keyRow(t, cl, id)
	if row["dailyLimit"] != float64(2) || row["usedToday"] != float64(2) || row["usedTotal"] != float64(2) {
		t.Errorf("the list shows the limit and the use: %v", row)
	}
}

func TestADailyLimitHasToBeSensible(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	for _, limit := range []any{0, -3, 1_000_001, "many", 2.5} {
		if _, _, r := makeKeyWith(t, cl, map[string]any{"dailyLimit": limit}); r.Status != http.StatusBadRequest {
			t.Errorf("dailyLimit %v: %d", limit, r.Status)
		}
	}
	if _, _, r := makeKeyWith(t, cl, map[string]any{"dailyLimit": 1}); r.Status != http.StatusCreated {
		t.Errorf("a limit of 1: %d", r.Status)
	}
}

func TestEveryKeyIsCountedEvenWithoutALimit(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id, key := makeKey(t, cl, "counted")
	for i := 0; i < 3; i++ {
		a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	}
	row := keyRow(t, cl, id)
	if row["usedToday"] != float64(3) || row["usedTotal"] != float64(3) || row["dailyLimit"] != nil {
		t.Errorf("counters: %v", row)
	}
	// yesterday's use counts in the total only
	a.db.Exec("INSERT INTO api_key_usage (key_id, day, requests) VALUES (?, CURRENT_DATE - 1, 10)", id)
	row = keyRow(t, cl, id)
	if row["usedToday"] != float64(3) || row["usedTotal"] != float64(13) {
		t.Errorf("counters with an earlier day: %v", row)
	}
}

func TestTheUsageRouteSaysWhatTheKeyMayDo(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key, _ := makeKeyWith(t, cl, map[string]any{"name": "reporting", "dailyLimit": 50, "expiresInDays": 10})
	a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	r := a.call(http.MethodGet, "/v1/usage", key, nil)
	k, _ := r.JSON()["key"].(map[string]any)
	if r.Status != http.StatusOK || k["name"] != "reporting" || k["usedToday"] != float64(1) || k["dailyLimit"] != float64(50) || k["expiresAt"] == nil {
		t.Errorf("usage: %d %s", r.Status, r.Body)
	}
	if _, ok := r.JSON()["summaries"]; !ok {
		t.Error("the owner's allowance is still shown")
	}
	// looking at the usage is not a request that is counted
	if k["usedToday"] != float64(1) {
		t.Errorf("usage counted itself: %v", k["usedToday"])
	}
}

func TestAFailedCallIsGivenBackToTheKeyAsWell(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"dailyLimit": 1})
	a.ai.fail.Store(true)
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusBadGateway {
		t.Fatalf("a failing AI service: %d", r.Status)
	}
	a.ai.fail.Store(false)
	if keyRow(t, cl, id)["usedToday"] != float64(0) {
		t.Error("a failed call is not counted against the key")
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusOK {
		t.Errorf("the key still has its one request: %d %s", r.Status, r.Body)
	}
}

func TestAKeyThatIsOutOfRequestsDoesNotUseUpTheOwnersAllowanceAndViceVersa(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "2")
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"dailyLimit": 10})
	a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	if r.Status != http.StatusTooManyRequests || r.Str("code") != "quota_exceeded" {
		t.Fatalf("the owner's allowance is spent: %d %s", r.Status, r.Body)
	}
	if keyRow(t, cl, id)["usedToday"] != float64(2) {
		t.Error("a request the owner's allowance refused is not counted for the key")
	}
}

func TestWhenTheSitesBudgetIsUsedUpEverythingClaimedIsGivenBack(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "1")
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"name": "late"})
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusOK {
		t.Fatalf("the one request the budget allows: %d", r.Status)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusServiceUnavailable || r.Str("code") != "daily_budget" {
		t.Fatalf("past the budget: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage"); n != 1 {
		t.Errorf("the owner lost allowance to a request that was never served: %d used, want 1", n)
	}
	if keyRow(t, cl, id)["usedToday"] != float64(1) {
		t.Error("the key was charged for a request that was never served")
	}
	// the same for the website
	if r := summarize(cl); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("the website is held too: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage"); n != 1 {
		t.Errorf("the website request used allowance: %d", n)
	}
}

func TestRevokingOrDeletingAKeyRemovesItsCounters(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	id, key := makeKey(t, cl, "gone")
	a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	if n := count(t, a.db, "SELECT count(*) FROM api_key_usage WHERE key_id = ?", id); n != 1 {
		t.Fatalf("counted: %d", n)
	}
	if r := cl.delete("/account", map[string]string{"confirmEmail": email, "password": goodPass}); r.Status != http.StatusOK {
		t.Fatalf("delete account: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT count(*) FROM api_key_usage"); n != 0 {
		t.Errorf("counters left behind: %d", n)
	}
}

// ---- saving to the library ------------------------------------------------------------------------------

func TestAResultIsSavedToTheLibraryOnlyWhenAskedFor(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key := makeKey(t, cl, "saver")

	plain := a.call(http.MethodPost, "/v1/summarize-text", key, textBody())
	if plain.Status != http.StatusOK || plain.JSON()["id"] != nil {
		t.Fatalf("not asked to save: %d %s", plain.Status, plain.Body)
	}
	if docs := listedDocuments(t, cl); len(docs) != 0 {
		t.Fatalf("nothing is saved by default: %v", docs)
	}

	saved := a.call(http.MethodPost, "/v1/summarize-text?save=true", key, textBody())
	id, _ := saved.JSON()["id"].(float64)
	if saved.Status != http.StatusOK || id == 0 || saved.Str("summary") == "" {
		t.Fatalf("saved: %d %s", saved.Status, saved.Body)
	}
	docs := listedDocuments(t, cl)
	if len(docs) != 1 || docs[0]["ID"] != id || docs[0]["hasContent"] != true {
		t.Errorf("the document is in the owner's library, with its text: %v", docs)
	}
	// so it can be used like any document: asked about, and compared
	if r := cl.post("/documents/"+itoa(int(id))+"/chat", map[string]string{"question": "hi"}); r.Status != http.StatusOK {
		t.Errorf("chat with a document saved through the API: %d %s", r.Status, r.Body)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text?save=false", key, textBody()); r.JSON()["id"] != nil {
		t.Error("save=false saves nothing")
	}
}

func TestSavedResultsCanBeStreamedAndOtherRoutesSaveToo(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key := makeKey(t, cl, "saver")

	r := a.call(http.MethodPost, "/v1/summarize-text?stream=true&save=true", key, textBody())
	if r.Status != http.StatusOK || !strings.Contains(string(r.Body), `"type":"done"`) || !strings.Contains(string(r.Body), `"id":`) {
		t.Fatalf("a streamed save: %d %s", r.Status, r.Body)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-url?save=true", key, map[string]string{"url": "https://news.example/post"}); r.JSON()["id"] == nil {
		t.Errorf("a link: %s", r.Body)
	}
	if r := a.callUpload("/v1/summarize?save=true", key, "file", map[string][]byte{"report.pdf": pdfBytes}); r.JSON()["id"] == nil {
		t.Errorf("a file: %s", r.Body)
	}
	if docs := listedDocuments(t, cl); len(docs) != 3 {
		t.Errorf("three documents saved: %d", len(docs))
	}
	if files := count(t, a.db, "SELECT count(*) FROM document_files"); files != 1 {
		t.Errorf("the PDF is kept like any saved file: %d", files)
	}
}

// ---- comparing and extracting from text ---------------------------------------------------------------------

func TestTwoTextsCanBeComparedThroughTheAPI(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"name": "diff"})
	body := map[string]any{
		"old":      map[string]string{"name": "v1", "text": "Payment is due within 30 days."},
		"new":      map[string]string{"name": "v2", "text": "Payment is due within 60 days."},
		"language": "German",
	}
	r := a.call(http.MethodPost, "/v1/compare", key, body)
	changes, _ := r.JSON()["changes"].([]any)
	if r.Status != http.StatusOK || len(changes) != 2 || r.JSON()["bottomLine"] == "" {
		t.Fatalf("compare: %d %s", r.Status, r.Body)
	}
	var sent struct {
		Old, New struct{ Name, Text string }
		Language string
	}
	raw, _ := a.ai.lastCompare.Load().([]byte)
	_ = json.Unmarshal(raw, &sent)
	if sent.Old.Name != "v1" || !strings.Contains(sent.New.Text, "60 days") || sent.Language != "German" {
		t.Errorf("sent: %s", raw)
	}
	if keyRow(t, cl, id)["usedToday"] != float64(1) || r.Header.Get("X-Quota-Remaining") == "" {
		t.Error("counted like any request")
	}
	if docs := listedDocuments(t, cl); len(docs) != 0 {
		t.Error("nothing is saved")
	}
}

func TestBadComparisonsAreRefusedAndNotCharged(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	id, key, _ := makeKeyWith(t, cl, map[string]any{"dailyLimit": 5})
	side := func(text string) map[string]string { return map[string]string{"name": "x", "text": text} }
	cases := map[string]struct {
		body map[string]any
		want int
	}{
		"nothing":      {map[string]any{}, http.StatusBadRequest},
		"an empty old": {map[string]any{"old": side("  "), "new": side("text")}, http.StatusBadRequest},
		"an empty new": {map[string]any{"old": side("text"), "new": side("")}, http.StatusBadRequest},
		"a language":   {map[string]any{"old": side("a"), "new": side("b"), "language": "Klingon"}, http.StatusBadRequest},
		"too long":     {map[string]any{"old": side(strings.Repeat("a", 300_001)), "new": side("b")}, http.StatusRequestEntityTooLarge},
	}
	for name, c := range cases {
		if r := a.call(http.MethodPost, "/v1/compare", key, c.body); r.Status != c.want {
			t.Errorf("%s: %d %s, want %d", name, r.Status, r.Body, c.want)
		}
	}
	a.ai.fail.Store(true)
	if r := a.call(http.MethodPost, "/v1/compare", key, map[string]any{"old": side("a"), "new": side("b")}); r.Status != http.StatusBadGateway {
		t.Errorf("a failing AI service: %d", r.Status)
	}
	a.ai.fail.Store(false)
	if n := a.ai.calls.Load(); n != 1 {
		t.Errorf("only the failing call reached the AI service: %d", n)
	}
	if keyRow(t, cl, id)["usedToday"] != float64(0) {
		t.Error("none of those counted for the key")
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage"); n != 0 {
		t.Errorf("none of those used the owner's allowance: %d", n)
	}
}

func TestFieldsCanBeExtractedFromTextThroughTheAPI(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key := makeKey(t, cl, "extractor")
	r := a.call(http.MethodPost, "/v1/extract", key, map[string]any{
		"name": "invoice.txt", "text": "Invoice number: INV-42. Total: 120 euros.",
		"fields": []map[string]string{{"name": "Invoice number"}, {"name": "Total", "type": "amount"}},
	})
	fields, _ := r.JSON()["fields"].([]any)
	if r.Status != http.StatusOK || len(fields) != 2 || r.JSON()["name"] != "invoice.txt" || r.JSON()["suspicious"] != false {
		t.Fatalf("extract: %d %s", r.Status, r.Body)
	}
	first := fields[0].(map[string]any)
	if first["name"] != "Invoice number" || first["verified"] != true {
		t.Errorf("first: %v", first)
	}
	if docs := listedDocuments(t, cl); len(docs) != 0 {
		t.Error("nothing is saved")
	}
}

func TestBadExtractionsAreRefusedWithoutAskingTheAIService(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key := makeKey(t, cl, "extractor")
	field := []map[string]string{{"name": "Total"}}
	cases := map[string]struct {
		body map[string]any
		want int
	}{
		"no fields":     {map[string]any{"text": "x", "fields": []map[string]string{}}, http.StatusBadRequest},
		"a bad field":   {map[string]any{"text": "x", "fields": []map[string]string{{"name": " "}}}, http.StatusBadRequest},
		"no text":       {map[string]any{"text": "  ", "fields": field}, http.StatusBadRequest},
		"a language":    {map[string]any{"text": "x", "fields": field, "language": "Klingon"}, http.StatusBadRequest},
		"text too long": {map[string]any{"text": strings.Repeat("a", 400_001), "fields": field}, http.StatusRequestEntityTooLarge},
	}
	for name, c := range cases {
		if r := a.call(http.MethodPost, "/v1/extract", key, c.body); r.Status != c.want {
			t.Errorf("%s: %d %s, want %d", name, r.Status, r.Body, c.want)
		}
	}
	if n := a.ai.calls.Load(); n != 0 {
		t.Errorf("the AI service must not be asked (%d calls)", n)
	}
}

func TestTheNewRoutesNeedAValidKeyAndHonourTheLimits(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	_, key, _ := makeKeyWith(t, cl, map[string]any{"dailyLimit": 1})
	text := map[string]any{"text": "x", "fields": []map[string]string{{"name": "Total"}}}
	for _, path := range []string{"/v1/compare", "/v1/extract"} {
		if r := a.call(http.MethodPost, path, "", text); r.Status != http.StatusUnauthorized {
			t.Errorf("%s without a key: %d", path, r.Status)
		}
		if r := cl.post(path, text); r.Status != http.StatusUnauthorized {
			t.Errorf("%s with a browser session: %d", path, r.Status)
		}
	}
	// one key, one limit, shared by every route
	if r := a.call(http.MethodPost, "/v1/extract", key, text); r.Status != http.StatusOK {
		t.Fatalf("the first: %d %s", r.Status, r.Body)
	}
	if r := a.call(http.MethodPost, "/v1/summarize-text", key, textBody()); r.Status != http.StatusTooManyRequests || r.Str("code") != "api_key_limit" {
		t.Errorf("the key's limit covers every route: %d %s", r.Status, r.Body)
	}
}
