package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Comparing two saved documents: the gateway checks they are the person's own and have text, asks the AI service,
// and passes on only a result of the expected shape.

func saveNamed(t *testing.T, a *app, cl *client, name, text string) uint {
	t.Helper()
	id := saveText(t, cl, text)
	if err := a.db.Exec("UPDATE documents SET filename = ? WHERE id = ?", name, id).Error; err != nil {
		t.Fatal(err)
	}
	return uint(id)
}

func compareRequest(cl *client, oldID, newID uint, extra ...string) reply {
	body := map[string]any{"oldId": oldID, "newId": newID}
	if len(extra) > 0 {
		body["language"] = extra[0]
	}
	return cl.post("/documents/compare", body)
}

func TestTwoOfMyDocumentsCanBeCompared(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	older := saveNamed(t, a, cl, "Contract v1.pdf", "Payment is due within 30 days of the invoice. The supplier delivers in 30 days.")
	newer := saveNamed(t, a, cl, "Contract v2.pdf", "Payment is due within 60 days of the invoice. The supplier delivers in 30 days.")

	r := compareRequest(cl, older, newer, "German")
	if r.Status != http.StatusOK {
		t.Fatalf("compare: %d %s", r.Status, r.Body)
	}
	out := r.JSON()
	changes, _ := out["changes"].([]any)
	if len(changes) != 2 || out["bottomLine"] == "" || out["explained"] != true {
		t.Errorf("the result: %s", r.Body)
	}
	first, _ := changes[0].(map[string]any)
	if first["kind"] != "changed" || first["importance"] != "high" || first["summary"] == "" {
		t.Errorf("the first change: %v", first)
	}

	// the AI service was given the two documents' names and texts, older first, and the language
	var sent struct {
		Old, New struct{ Name, Text string }
		Language string
	}
	raw, _ := a.ai.lastCompare.Load().([]byte)
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Old.Name != "Contract v1.pdf" || !strings.Contains(sent.Old.Text, "30 days of the invoice") ||
		sent.New.Name != "Contract v2.pdf" || !strings.Contains(sent.New.Text, "60 days of the invoice") || sent.Language != "German" {
		t.Errorf("sent to the AI service: %s", raw)
	}
	// nothing is saved by comparing
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 2 {
		t.Errorf("a comparison must not add documents: %d", n)
	}
}

func TestComparingUsesOneSummaryOfTheAllowanceAndTheSitesBudget(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "10")
	cl, _ := a.newUser()
	older := saveNamed(t, a, cl, "a", "First version of the text.")
	newer := saveNamed(t, a, cl, "b", "Second version of the text.")
	used := func() int {
		return count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage")
	}
	before := used()
	if r := compareRequest(cl, older, newer); r.Status != http.StatusOK {
		t.Fatalf("compare: %d", r.Status)
	}
	if got := used() - before; got != 1 {
		t.Errorf("one comparison is one summary of the allowance, used %d", got)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(requests), 0) FROM global_usage"); n < 1 {
		t.Errorf("the site's budget must count it: %d", n)
	}
}

func TestWhatCannotBeComparedIsRefusedAndNotCharged(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	mine := saveNamed(t, a, cl, "mine", "My own document text here.")
	other := saveNamed(t, a, cl, "other", "Another of my documents here.")
	bob, _ := a.newUser()
	bobs := saveNamed(t, a, bob, "bob's", "Bob's private document text.")
	empty := saveNamed(t, a, cl, "old one", "Text that is about to be removed.")
	a.db.Exec("UPDATE documents SET has_content = false, content = '' WHERE id = ?", empty)

	callsBefore := a.ai.calls.Load()
	cases := map[string]struct {
		r    reply
		want int
	}{
		"nothing chosen":          {cl.post("/documents/compare", map[string]any{}), http.StatusBadRequest},
		"one missing":             {compareRequest(cl, mine, 0), http.StatusBadRequest},
		"the same twice":          {compareRequest(cl, mine, mine), http.StatusBadRequest},
		"someone else's document": {compareRequest(cl, mine, bobs), http.StatusNotFound},
		"a document that is gone": {compareRequest(cl, mine, 999999), http.StatusNotFound},
		"no saved text":           {compareRequest(cl, mine, empty), http.StatusConflict},
		"a language that is none": {compareRequest(cl, mine, other, "Klingon"), http.StatusBadRequest},
		"bad json":                {cl.post("/documents/compare", "not json"), http.StatusBadRequest},
	}
	for name, c := range cases {
		if c.r.Status != c.want {
			t.Errorf("%s: %d %s, want %d", name, c.r.Status, c.r.Body, c.want)
		}
	}
	if strings.Contains(string(cases["someone else's document"].r.Body), "Bob") {
		t.Error("nothing about another person's document may be revealed")
	}
	if n := a.ai.calls.Load() - callsBefore; n != 0 {
		t.Errorf("the AI service must not be asked for a refused comparison (%d calls)", n)
	}
}

func TestARefusedComparisonDoesNotUseTheAllowance(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "4")
	cl, _ := a.newUser()
	one := saveNamed(t, a, cl, "one", "First text of the document.")
	two := saveNamed(t, a, cl, "two", "Second text of the document.")
	// two summaries were used saving them; refused and failed comparisons must not use the other two
	for i := 0; i < 5; i++ {
		if r := compareRequest(cl, one, one); r.Status != http.StatusBadRequest {
			t.Fatalf("same twice: %d", r.Status)
		}
	}
	a.ai.fail.Store(true)
	for i := 0; i < 3; i++ {
		if r := compareRequest(cl, one, two); r.Status != http.StatusBadGateway {
			t.Fatalf("a failing AI service: %d %s", r.Status, r.Body)
		}
	}
	a.ai.fail.Store(false)
	if r := compareRequest(cl, one, two); r.Status != http.StatusOK {
		t.Errorf("the allowance must still be there: %d %s", r.Status, r.Body)
	}
}

func TestOnlyTheExpectedShapeOfAComparisonReachesTheBrowser(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	one := saveNamed(t, a, cl, "one", "First text of the document.")
	two := saveNamed(t, a, cl, "two", "Second text of the document.")
	long := strings.Repeat("x", 9000)
	a.ai.compareReply.Store(`{"identical":false,"secret":"leak","counts":{"added":2,"hacked":9,"changed":-4},"changes":[` +
		`{"id":1,"kind":"exploit","before":"a","after":"b","importance":"high","summary":"dropped: unknown kind"},` +
		`{"id":2,"kind":"changed","before":"` + long + `","after":"b","importance":"<script>","summary":"` + long + `","impact":"i",` +
		`"segments":[["eq","a"],["boom","b"]],"numbers":{"removed":["1","2","3","4","5","6","7","8","9","10","11","12"],"added":[]},"explained":true,"extra":"x"},` +
		`{"id":3,"kind":"added","before":"","after":"ok","importance":"low","summary":"fine","impact":"","segments":[["eq","a"],["ins","b"]],` +
		`"numbers":{"removed":[],"added":[]},"explained":false}],"bottomLine":"` + long + `","explained":true,"omitted":-5}`)

	r := compareRequest(cl, one, two)
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if strings.Contains(string(r.Body), "leak") || strings.Contains(string(r.Body), "hacked") || strings.Contains(string(r.Body), "<script>") {
		t.Errorf("unexpected content got through: %.300s", r.Body)
	}
	out := r.JSON()
	counts, _ := out["counts"].(map[string]any)
	if counts["added"] != float64(2) || counts["changed"] != float64(0) || counts["moved"] != float64(0) || len(counts) != 4 {
		t.Errorf("counts: %v", counts)
	}
	if out["omitted"] != float64(0) || len([]rune(out["bottomLine"].(string))) > 1200 {
		t.Errorf("omitted and the bottom line: %v", out)
	}
	changes := out["changes"].([]any)
	if len(changes) != 2 {
		t.Fatalf("a change of an unknown kind is dropped: %s", r.Body)
	}
	second := changes[0].(map[string]any)
	if second["importance"] != "low" || second["segments"] != nil || len([]rune(second["before"].(string))) > 3000 || len([]rune(second["summary"].(string))) > 400 {
		t.Errorf("the second change: importance %v, segments %v", second["importance"], second["segments"])
	}
	if removed := second["numbers"].(map[string]any)["removed"].([]any); len(removed) != 10 {
		t.Errorf("at most 10 numbers per change: %d", len(removed))
	}
	third := changes[1].(map[string]any)
	if segs, _ := third["segments"].([]any); len(segs) != 2 {
		t.Errorf("good segments are kept: %v", third["segments"])
	}
}

func TestAnUnreadableReplyFromTheAIServiceIsAPlainErrorAndIsNotCharged(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	one := saveNamed(t, a, cl, "one", "First text of the document.")
	two := saveNamed(t, a, cl, "two", "Second text of the document.")
	a.ai.compareReply.Store(`this is not json`)
	r := compareRequest(cl, one, two)
	if r.Status != http.StatusBadGateway || strings.Contains(string(r.Body), "not json") {
		t.Errorf("%d %s", r.Status, r.Body)
	}
	a.ai.compareReply.Store("")
	if r := compareRequest(cl, one, two); r.Status != http.StatusOK {
		t.Errorf("not charged for the failure: %d %s", r.Status, r.Body)
	}
}

func TestComparingNeedsAnAccount(t *testing.T) {
	a := newApp(t)
	if r := a.newClient().post("/documents/compare", map[string]any{"oldId": 1, "newId": 2}); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
	cl, _ := a.newUser()
	one := saveNamed(t, a, cl, "one", "First text of the document.")
	two := saveNamed(t, a, cl, "two", "Second text of the document.")
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "true")
	if r := compareRequest(cl, one, two); r.Status != http.StatusForbidden {
		t.Errorf("an unverified email: %d", r.Status)
	}
}
