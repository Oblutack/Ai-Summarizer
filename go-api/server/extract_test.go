package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Extracting fields from a saved document: the gateway checks the request and the document, asks the AI service, and
// passes on only results of the expected shape, for the fields that were asked for.

func extractRequest(cl *client, id uint, fields any, extra ...string) reply {
	body := map[string]any{"fields": fields}
	if len(extra) > 0 {
		body["language"] = extra[0]
	}
	return cl.post("/documents/"+itoa(int(id))+"/extract", body)
}

func someFields() []map[string]string {
	return []map[string]string{
		{"name": "Invoice number", "description": "as printed", "type": "text"},
		{"name": "Total", "type": "amount"},
	}
}

func TestFieldsCanBeExtractedFromMyDocument(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "invoice.pdf", "Invoice number: INV-42. Total: 120 euros.")

	r := extractRequest(cl, id, someFields(), "German")
	if r.Status != http.StatusOK {
		t.Fatalf("extract: %d %s", r.Status, r.Body)
	}
	out := r.JSON()
	fields, _ := out["fields"].([]any)
	if out["name"] != "invoice.pdf" || len(fields) != 2 {
		t.Fatalf("the result: %s", r.Body)
	}
	first := fields[0].(map[string]any)
	if first["name"] != "Invoice number" || first["value"] != "Value of Invoice number" || first["verified"] != true || first["page"] != float64(1) {
		t.Errorf("the first field: %v", first)
	}

	var sent struct {
		Name, Text string
		Language   string
		Fields     []struct{ Name, Description, Type string }
	}
	raw, _ := a.ai.lastExtract.Load().([]byte)
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Name != "invoice.pdf" || !strings.Contains(sent.Text, "INV-42") || sent.Language != "German" || len(sent.Fields) != 2 ||
		sent.Fields[0].Description != "as printed" || sent.Fields[1].Type != "amount" {
		t.Errorf("sent to the AI service: %s", raw)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 1 {
		t.Errorf("extracting must not add documents: %d", n)
	}
}

func TestTheFieldsAreTidiedBeforeTheyAreSent(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	r := extractRequest(cl, id, []map[string]string{
		{"name": "  Invoice \n  number ", "description": "  as\tprinted \u0007 here ", "type": "TEXT"},
		{"name": "Due date"},
	})
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	var sent struct {
		Fields []struct{ Name, Description, Type string }
	}
	raw, _ := a.ai.lastExtract.Load().([]byte)
	_ = json.Unmarshal(raw, &sent)
	if sent.Fields[0].Name != "Invoice number" || sent.Fields[0].Description != "as printed here" || sent.Fields[0].Type != "text" || sent.Fields[1].Type != "text" {
		t.Errorf("tidied fields: %+v", sent.Fields)
	}
}

func TestBadRequestsAreRefusedWithoutAskingTheAIServiceOrChargingAnything(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.") // saving it and the empty one below use two of the three
	bob, _ := a.newUser()
	bobs := saveNamed(t, a, bob, "bobs", "Bob's private text.")
	empty := saveNamed(t, a, cl, "empty", "Text to be removed.")
	a.db.Exec("UPDATE documents SET has_content = false, content = '' WHERE id = ?", empty)
	tooMany := make([]map[string]string, 21)
	for i := range tooMany {
		tooMany[i] = map[string]string{"name": "field " + itoa(i)}
	}

	callsBefore := a.ai.calls.Load()
	cases := map[string]struct {
		r    reply
		want int
	}{
		"no fields":               {extractRequest(cl, id, []map[string]string{}), http.StatusBadRequest},
		"too many fields":         {extractRequest(cl, id, tooMany), http.StatusBadRequest},
		"a nameless field":        {extractRequest(cl, id, []map[string]string{{"name": "  "}}), http.StatusBadRequest},
		"a long name":             {extractRequest(cl, id, []map[string]string{{"name": strings.Repeat("x", 61)}}), http.StatusBadRequest},
		"the same field twice":    {extractRequest(cl, id, []map[string]string{{"name": "Total"}, {"name": "total"}}), http.StatusBadRequest},
		"a type that is none":     {extractRequest(cl, id, []map[string]string{{"name": "Total", "type": "spreadsheet"}}), http.StatusBadRequest},
		"a language that is none": {extractRequest(cl, id, someFields(), "Klingon"), http.StatusBadRequest},
		"bad json":                {cl.post("/documents/"+itoa(int(id))+"/extract", "nope"), http.StatusBadRequest},
		"someone else's document": {extractRequest(cl, bobs, someFields()), http.StatusNotFound},
		"a document that is gone": {extractRequest(cl, 999999, someFields()), http.StatusNotFound},
		"no saved text":           {extractRequest(cl, empty, someFields()), http.StatusConflict},
		"a bad id":                {cl.post("/documents/abc/extract", map[string]any{"fields": someFields()}), http.StatusBadRequest},
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
		t.Errorf("the AI service must not be asked (%d calls)", n)
	}
	// none of those used the allowance: one more really works (the third of the three allowed)
	if r := extractRequest(cl, id, someFields()); r.Status != http.StatusOK {
		t.Errorf("the allowance must still be there: %d %s", r.Status, r.Body)
	}
}

func TestExtractingUsesOneSummaryOfTheAllowanceAndTheSitesBudget(t *testing.T) {
	a := newApp(t)
	t.Setenv("AI_REQUESTS_PER_DAY", "10")
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	before := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage")
	if r := extractRequest(cl, id, someFields()); r.Status != http.StatusOK {
		t.Fatalf("%d", r.Status)
	}
	if got := count(t, a.db, "SELECT COALESCE(SUM(summaries), 0) FROM daily_usage") - before; got != 1 {
		t.Errorf("one extraction is one summary of the allowance, used %d", got)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(requests), 0) FROM global_usage"); n < 1 {
		t.Errorf("the site's budget must count it: %d", n)
	}
}

func TestAFailedExtractionIsGivenBackAndShowsNoInternals(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.") // 1 of 3
	a.ai.fail.Store(true)
	for i := 0; i < 3; i++ {
		r := extractRequest(cl, id, someFields())
		if r.Status != http.StatusBadGateway || strings.Contains(string(r.Body), "traceback") {
			t.Fatalf("a failing AI service: %d %s", r.Status, r.Body)
		}
	}
	a.ai.fail.Store(false)
	if r := extractRequest(cl, id, someFields()); r.Status != http.StatusOK {
		t.Errorf("not charged for failures: %d %s", r.Status, r.Body)
	}
}

func TestOnlyTheExpectedShapeOfAnExtractionReachesTheBrowser(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	long := strings.Repeat("x", 3000)
	a.ai.extractReply.Store(`{"name":"leak","secret":"leak","fields":[` +
		`{"name":"Renamed by the AI","type":"exploit","value":"` + long + `","quote":"` + long + `","page":0,"found":true,"verified":true,"reason":"` + long + `","extra":"x"},` +
		`{"name":"Total","type":"amount","value":"stray value","quote":"stray quote","page":3,"found":false,"verified":true,"reason":"stray"}]}`)
	r := extractRequest(cl, id, someFields())
	if r.Status != http.StatusOK {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if strings.Contains(string(r.Body), "leak") || strings.Contains(string(r.Body), "Renamed") || strings.Contains(string(r.Body), "stray") {
		t.Errorf("unexpected content got through: %.300s", r.Body)
	}
	fields := r.JSON()["fields"].([]any)
	first, second := fields[0].(map[string]any), fields[1].(map[string]any)
	if first["name"] != "Invoice number" || first["type"] != "text" {
		t.Errorf("names and types are the ones asked for: %v", first)
	}
	if len([]rune(first["value"].(string))) > 400 || len([]rune(first["quote"].(string))) > 300 || len([]rune(first["reason"].(string))) > 200 || first["page"] != nil {
		t.Errorf("values are cut to size and a page below 1 is dropped: %v", first["page"])
	}
	if second["found"] != false || second["value"] != "" || second["quote"] != "" || second["verified"] != false || second["reason"] != "" {
		t.Errorf("a field that was not found carries nothing and is not verified: %v", second)
	}
}

func TestADocumentWithInstructionsInItIsFlaggedToTheBrowser(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	if r := extractRequest(cl, id, someFields()); r.JSON()["suspicious"] != false {
		t.Errorf("an ordinary document: %s", r.Body)
	}
	a.ai.extractReply.Store(`{"suspicious":true,"fields":[{"name":"a","type":"text","value":"v","quote":"q","page":null,"found":true,"verified":false,"reason":"The quote comes from text that reads like an instruction to an AI."},` +
		`{"name":"b","type":"amount","value":"","quote":"","page":null,"found":false,"verified":false,"reason":""}]}`)
	r := extractRequest(cl, id, someFields())
	if r.Status != http.StatusOK || r.JSON()["suspicious"] != true {
		t.Errorf("the warning must reach the page: %d %s", r.Status, r.Body)
	}
}

func TestAnUnreadableOrIncompleteReplyIsAPlainErrorAndIsNotCharged(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	for _, bad := range []string{`not json`, `{"fields":[]}`, `{"fields":[{"name":"Invoice number"}]}`} {
		a.ai.extractReply.Store(bad)
		r := extractRequest(cl, id, someFields())
		if r.Status != http.StatusBadGateway {
			t.Errorf("%q: %d %s", bad, r.Status, r.Body)
		}
	}
	a.ai.extractReply.Store("")
	if r := extractRequest(cl, id, someFields()); r.Status != http.StatusOK {
		t.Errorf("not charged: %d", r.Status)
	}
}

func TestExtractingNeedsAnAccountAndAVerifiedEmailWhenRequired(t *testing.T) {
	a := newApp(t)
	if r := a.newClient().post("/documents/1/extract", map[string]any{"fields": someFields()}); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
	cl, _ := a.newUser()
	id := saveNamed(t, a, cl, "doc", "Some text of the document.")
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "true")
	if r := extractRequest(cl, id, someFields()); r.Status != http.StatusForbidden {
		t.Errorf("an unverified email: %d", r.Status)
	}
}
