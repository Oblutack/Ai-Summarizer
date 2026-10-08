package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The overview of a collection: one briefing written from the saved summaries of the documents that
// share a tag.

func overviewOf(cl *client, tag string, query ...string) reply {
	path := "/library/overview"
	if len(query) > 0 {
		path += "?" + query[0]
	}
	return cl.post(path, map[string]string{"tag": tag})
}

type overviewBody struct {
	Name      string `json:"name"`
	Documents []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"documents"`
}

// sentOverview decodes the last request the AI service got: its query string and its body.
func sentOverview(t *testing.T, a *app) (string, overviewBody) {
	t.Helper()
	var body overviewBody
	raw, _ := a.ai.lastSummary.Load().(string)
	query, payload, _ := strings.Cut(raw, "\n")
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		t.Fatalf("the AI service did not get an overview request: %v (%q)", err, raw)
	}
	return query, body
}

func collectionOf(t *testing.T, cl *client, tag string, texts ...string) []int {
	t.Helper()
	var ids []int
	for _, text := range texts {
		id := saveText(t, cl, text)
		tagDocument(t, cl, id, tag)
		ids = append(ids, id)
	}
	return ids
}

func TestAnOverviewIsWrittenFromTheSummariesOfTheCollection(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	collectionOf(t, cl, "Biology Course", "Lecture one about cells and nuclei.", "Lecture two about photosynthesis.", "Lecture three about genetics.")
	saveText(t, cl, "A lease agreement that has nothing to do with biology.")

	r := overviewOf(cl, "  BIOLOGY   course ")
	if r.Status != http.StatusOK || r.Str("summary") == "" {
		t.Fatalf("overview: %d %s", r.Status, r.Body)
	}
	query, body := sentOverview(t, a)
	if body.Name != "biology course" || len(body.Documents) != 3 {
		t.Fatalf("the AI service should get the collection and its three documents: %+v", body)
	}
	// Oldest first, and only the collection's documents, with their summaries (the fake AI writes "fake summary (...)").
	for i, want := range []string{"Lecture one", "Lecture two", "Lecture three"} {
		if !strings.HasPrefix(body.Documents[i].Name, want) || !strings.HasPrefix(body.Documents[i].Text, "fake summary") {
			t.Errorf("document %d: %+v", i, body.Documents[i])
		}
	}
	// A longer default length than a one-document summary, and no page limit.
	if !strings.Contains(query, "word_count=300") || strings.Contains(query, "page_limit") {
		t.Errorf("query = %q", query)
	}
	// Nothing was saved: an overview is shown, not stored.
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 4 {
		t.Errorf("documents = %d, an overview must not be saved as one", n)
	}
}

func TestAnOverviewHonoursTheChosenOptionsAndCanBeStreamed(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	collectionOf(t, cl, "course", "First lecture text.", "Second lecture text.")

	r := cl.post("/library/overview?wordCount=500&style=bullets&language=German&stream=true", map[string]string{"tag": "course"})
	if r.Status != http.StatusOK || !strings.Contains(string(r.Body), `"type":"done"`) {
		t.Fatalf("stream: %d %s", r.Status, r.Body)
	}
	query, _ := sentOverview(t, a)
	for _, want := range []string{"word_count=500", "style=bullets", "language=German", "stream=true"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q lacks %s", query, want)
		}
	}
}

func TestAnOverviewNeedsAtLeastTwoDocuments(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "2")
	cl, _ := a.newUser()
	collectionOf(t, cl, "lonely", "The only document with this tag.")
	used := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage")

	for _, tag := range []string{"lonely", "no-such-tag"} {
		if r := overviewOf(cl, tag); r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "at least two documents") {
			t.Errorf("%s: %d %s", tag, r.Status, r.Body)
		}
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != used {
		t.Errorf("a refused overview must not use the allowance: %d -> %d", used, n)
	}
}

func TestADeletedDocumentIsLeftOutOfTheOverview(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	ids := collectionOf(t, cl, "course", "Kept lecture one.", "Kept lecture two.", "Deleted lecture three.")
	if r := cl.delete(docPath(ids[2], ""), nil); r.Status != http.StatusOK && r.Status != http.StatusNoContent {
		t.Fatalf("delete: %d", r.Status)
	}
	if r := overviewOf(cl, "course"); r.Status != http.StatusOK {
		t.Fatalf("overview: %d %s", r.Status, r.Body)
	}
	if _, body := sentOverview(t, a); len(body.Documents) != 2 {
		t.Errorf("a deleted document must not be summarized: %+v", body.Documents)
	}
}

func TestAnOverviewOnlyEverUsesTheUsersOwnDocuments(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	collectionOf(t, alice, "notes", "Alice private note one.", "Alice private note two.")
	collectionOf(t, bob, "notes", "Bob note one.")

	// Bob has one document with the tag; Alice's two must not make his collection big enough.
	if r := overviewOf(bob, "notes"); r.Status != http.StatusBadRequest {
		t.Fatalf("Bob has a collection of one: %d %s", r.Status, r.Body)
	}
	collectionOf(t, bob, "notes", "Bob note two.")
	if r := overviewOf(bob, "notes"); r.Status != http.StatusOK {
		t.Fatalf("overview: %d %s", r.Status, r.Body)
	}
	_, body := sentOverview(t, a)
	if len(body.Documents) != 2 {
		t.Fatalf("expected Bob's two documents: %+v", body.Documents)
	}
	for _, d := range body.Documents {
		if strings.Contains(d.Name, "Alice") {
			t.Errorf("another person's document leaked into the overview: %+v", d)
		}
	}
}

func TestAnOverviewNeedsASignedInUserAndAValidRequest(t *testing.T) {
	a := newApp(t)
	if r := overviewOf(a.newClient(), "course"); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
	cl, _ := a.newUser()
	for name, tag := range map[string]string{"empty": "", "blank": "   ", "too long": strings.Repeat("x", 200)} {
		if r := overviewOf(cl, tag); r.Status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	if r := cl.post("/library/overview", map[string]int{"tag": 5}); r.Status != http.StatusBadRequest {
		t.Errorf("a malformed body: %d", r.Status)
	}
	if r := overviewOf(cl, "course", "style=poem"); r.Status != http.StatusBadRequest {
		t.Errorf("an unknown style: %d", r.Status)
	}
}

func TestAFailedOverviewIsRefunded(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3") // the two summaries made below, and room for one overview
	cl, _ := a.newUser()
	collectionOf(t, cl, "course", "First lecture text.", "Second lecture text.")
	before := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage")

	a.ai.fail.Store(true)
	for i := 0; i < 4; i++ {
		if r := overviewOf(cl, "course"); r.Status != http.StatusBadGateway {
			t.Fatalf("AI failure should be a 502, got %d", r.Status)
		}
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != before {
		t.Errorf("failed overviews must not use up the allowance: %d -> %d", before, n)
	}
}

func TestAHugeSummaryIsCutBeforeItIsSent(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	ids := collectionOf(t, cl, "course", "First lecture text.", "Second lecture text.")
	a.db.Exec("UPDATE documents SET summary = ? WHERE id = ?", strings.Repeat("word ", 5000), ids[0])
	if r := overviewOf(cl, "course"); r.Status != http.StatusOK {
		t.Fatalf("overview: %d %s", r.Status, r.Body)
	}
	_, body := sentOverview(t, a)
	if got := len([]rune(body.Documents[0].Text)); got != 4000 {
		t.Errorf("a summary is cut to 4000 characters, got %d", got)
	}
}
