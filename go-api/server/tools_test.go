package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// Tests for what a person can do with a saved document: rename, tag, search, rewrite, share, email,
// questions to ask, flashcards and quizzes, and standing instructions.

func docPath(id int, suffix string) string { return "/documents/" + strconv.Itoa(id) + suffix }

func titleOf(t *testing.T, cl *client, id int) string {
	t.Helper()
	for _, d := range listedDocuments(t, cl) {
		if int(d["ID"].(float64)) == id {
			return d["Filename"].(string)
		}
	}
	t.Fatalf("document %d is not listed", id)
	return ""
}

// ---- rename and tags ---------------------------------------------------------------------------

func TestADocumentCanBeRenamed(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")

	r := cl.put(docPath(id, ""), map[string]string{"filename": "  Lease   for\tHarbour Street  "})
	if r.Status != http.StatusOK || r.Str("Filename") != "Lease for Harbour Street" {
		t.Fatalf("rename: %d %s", r.Status, r.Body)
	}
	if got := titleOf(t, cl, id); got != "Lease for Harbour Street" {
		t.Errorf("the new title is kept: %q", got)
	}
}

func TestBadTitlesAndEmptyChangesAreRefused(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")
	before := titleOf(t, cl, id)

	for name, body := range map[string]any{
		"blank":    map[string]string{"filename": "   \n "},
		"too long": map[string]string{"filename": strings.Repeat("x", 201)},
		"nothing":  map[string]string{},
	} {
		if r := cl.put(docPath(id, ""), body); r.Status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	if got := titleOf(t, cl, id); got != before {
		t.Errorf("a refused change must change nothing: %q", got)
	}
	if r := cl.put("/documents/abc", map[string]string{"filename": "x"}); r.Status != http.StatusBadRequest {
		t.Errorf("a bad id: %d", r.Status)
	}
}

func TestTagsAreCleanedAndKept(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")

	r := cl.put(docPath(id, ""), map[string]any{"tags": []string{"  Legal ", "legal", "Housing  Costs", ""}})
	if r.Status != http.StatusOK {
		t.Fatalf("tags: %d %s", r.Status, r.Body)
	}
	got := listedDocuments(t, cl)[0]["tags"].([]any)
	if len(got) != 2 || got[0] != "legal" || got[1] != "housing costs" {
		t.Errorf("tags are lowercased, trimmed and not repeated: %v", got)
	}

	if r := cl.put(docPath(id, ""), map[string]any{"tags": []string{}}); r.Status != http.StatusOK {
		t.Fatalf("clearing tags: %d", r.Status)
	}
	if got := listedDocuments(t, cl)[0]["tags"].([]any); len(got) != 0 {
		t.Errorf("tags are cleared: %v", got)
	}
}

func TestTooManyOrTooLongTagsAreRefused(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")

	many := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"}
	if r := cl.put(docPath(id, ""), map[string]any{"tags": many}); r.Status != http.StatusBadRequest {
		t.Errorf("nine tags: %d", r.Status)
	}
	if r := cl.put(docPath(id, ""), map[string]any{"tags": []string{strings.Repeat("x", 31)}}); r.Status != http.StatusBadRequest {
		t.Errorf("a long tag: %d", r.Status)
	}
}

func TestOneUserCannotChangeAnothersDocument(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's private words.")
	before := titleOf(t, alice, id)

	if r := bob.put(docPath(id, ""), map[string]string{"filename": "Hijacked"}); r.Status != http.StatusNotFound {
		t.Errorf("rename: %d", r.Status)
	}
	if r := bob.put(docPath(id, ""), map[string]any{"tags": []string{"mine"}}); r.Status != http.StatusNotFound {
		t.Errorf("tags: %d", r.Status)
	}
	if got := titleOf(t, alice, id); got != before {
		t.Errorf("Alice's title changed to %q", got)
	}
}

func TestTagsAreListedByUseAndFilterTheDocuments(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	other, _ := a.newUser()
	first := saveText(t, cl, "First document.")
	second := saveText(t, cl, "Second document.")
	third := saveText(t, cl, "Third document.")
	cl.put(docPath(first, ""), map[string]any{"tags": []string{"work", "legal"}})
	cl.put(docPath(second, ""), map[string]any{"tags": []string{"work"}})
	otherID := saveText(t, other, "Someone else's.")
	other.put(docPath(otherID, ""), map[string]any{"tags": []string{"work", "secret"}})

	var tags []map[string]any
	if err := jsonUnmarshal(cl.get("/documents/tags").Body, &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0]["tag"] != "work" || tags[0]["count"] != float64(2) || tags[1]["tag"] != "legal" {
		t.Errorf("my tags, most used first, and only mine: %v", tags)
	}

	var ids []int
	for _, d := range cl.getDocs(t, "?tag=Work") {
		ids = append(ids, int(d["ID"].(float64)))
	}
	if len(ids) != 2 || ids[0] != second || ids[1] != first {
		t.Errorf("the work documents, newest first: %v (third is %d)", ids, third)
	}
	if docs := cl.getDocs(t, "?tag=nothing"); len(docs) != 0 {
		t.Errorf("an unused tag finds nothing: %v", docs)
	}
}

func (cl *client) getDocs(t *testing.T, query string) []map[string]any {
	t.Helper()
	r := cl.get("/documents" + query)
	if r.Status != http.StatusOK {
		t.Fatalf("list %s: %d %s", query, r.Status, r.Body)
	}
	var docs []map[string]any
	if err := jsonUnmarshal(r.Body, &docs); err != nil {
		t.Fatal(err)
	}
	return docs
}

// ---- search --------------------------------------------------------------------------------------

func TestDocumentsAreSearchedByTitleAndSummaryWithoutPatterns(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	other, _ := a.newUser()
	plain := saveText(t, cl, "Just words.")
	percent := saveText(t, cl, "More words.")
	cl.put(docPath(percent, ""), map[string]string{"filename": "100% sure_thing"})
	saveText(t, other, "Words about a Zeppelin.")
	otherID := int(listedDocuments(t, other)[0]["ID"].(float64))
	other.put(docPath(otherID, ""), map[string]string{"filename": "Zeppelin plans"})

	titles := func(q string) []string {
		var out []string
		for _, d := range cl.getDocs(t, "?q="+url.QueryEscape(q)) {
			out = append(out, d["Filename"].(string))
		}
		return out
	}
	if got := titles("sure_"); len(got) != 1 || got[0] != "100% sure_thing" {
		t.Errorf("an underscore is a character, not a wildcard: %v", got)
	}
	if got := titles("%"); len(got) != 1 || got[0] != "100% sure_thing" {
		t.Errorf("a percent sign is a character, not a wildcard: %v", got)
	}
	if got := titles("SURE"); len(got) != 1 {
		t.Errorf("search ignores case: %v", got)
	}
	if got := titles("fake summary"); len(got) != 2 {
		t.Errorf("search covers the summary too: %v (plain is %d)", got, plain)
	}
	if got := titles("zeppelin"); len(got) != 0 {
		t.Errorf("never another user's documents: %v", got)
	}
	if r := cl.get("/documents?q=" + strings.Repeat("x", 101)); r.Status != http.StatusBadRequest {
		t.Errorf("a very long search: %d", r.Status)
	}
}

func TestSearchAndPagingWorkTogether(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	for i := 0; i < 3; i++ {
		saveText(t, cl, "Words number "+strconv.Itoa(i)+".")
	}
	r := cl.get("/documents?q=words&limit=2")
	if r.Status != http.StatusOK || r.Header.Get("X-Next-Cursor") == "" {
		t.Fatalf("a first page with a cursor: %d %q", r.Status, r.Header.Get("X-Next-Cursor"))
	}
}

// ---- rewrite ---------------------------------------------------------------------------------------

func TestADocumentCanBeSummarizedAgainInAnotherStyle(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "The tenant pays 950 euros per month.")
	before := listedDocuments(t, cl)[0]["Summary"]

	r := cl.post(docPath(id, "/rewrite"), map[string]any{"style": "bullets", "wordCount": 50, "language": "Spanish"})
	if r.Status != http.StatusOK {
		t.Fatalf("rewrite: %d %s", r.Status, r.Body)
	}
	got := listedDocuments(t, cl)[0]["Summary"].(string)
	if got == before || !strings.Contains(got, "style=bullets") || !strings.Contains(got, "word_count=50") || !strings.Contains(got, "language=Spanish") {
		t.Errorf("the stored summary is the new one, made with the new options: %q", got)
	}
	if r.Str("Summary") != got {
		t.Errorf("and it is what the request returns: %s", r.Body)
	}
	if sent, _ := a.ai.lastSummary.Load().(string); !strings.Contains(sent, "The tenant pays 950 euros per month.") {
		t.Errorf("the saved source text was summarized: %q", sent)
	}
}

func TestRewritingDropsWhatWasMadeFromTheOldSummary(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words about leases.")
	cl.post(docPath(id, "/podcast"), map[string]string{})
	cl.post(docPath(id, "/suggestions"), nil)
	cl.post(docPath(id, "/study"), map[string]string{"kind": "quiz"})
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE podcast IS NOT NULL AND suggestions IS NOT NULL AND study IS NOT NULL"); n != 1 {
		t.Fatalf("setup: %d", n)
	}

	if r := cl.post(docPath(id, "/rewrite"), map[string]any{"style": "brief"}); r.Status != http.StatusOK {
		t.Fatalf("rewrite: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE podcast IS NULL AND suggestions IS NULL AND study IS NULL"); n != 1 {
		t.Errorf("the podcast, questions and study material belonged to the old summary: %d", n)
	}
}

func TestRewritingValidatesAndNeedsTheOriginalText(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")
	for name, body := range map[string]map[string]any{
		"style":    {"style": "poem"},
		"language": {"language": "Klingon"},
		"words":    {"wordCount": 5},
	} {
		if r := cl.post(docPath(id, "/rewrite"), body); r.Status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	a.db.Exec("UPDATE documents SET has_content = false, content = '' WHERE id = ?", id)
	if r := cl.post(docPath(id, "/rewrite"), map[string]any{"style": "brief"}); r.Status != http.StatusConflict {
		t.Errorf("a document without its text: %d %s", r.Status, r.Body)
	}
}

func TestRewritingCountsAsASummaryAndAFailureIsRefunded(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "3")
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")                                                             // 1 of 3
	if r := cl.post(docPath(id, "/rewrite"), map[string]any{"style": "brief"}); r.Status != http.StatusOK { // 2 of 3
		t.Fatalf("rewrite: %d", r.Status)
	}
	a.ai.fail.Store(true)
	if r := cl.post(docPath(id, "/rewrite"), map[string]any{"style": "brief"}); r.Status < 500 {
		t.Fatalf("a failing AI service: %d", r.Status)
	}
	a.ai.fail.Store(false)
	if n := count(t, a.db, "SELECT COALESCE(SUM(summaries),0) FROM daily_usage"); n != 2 {
		t.Errorf("only the two that worked are charged, got %d", n)
	}
}

func TestOneUserCannotRewriteAnothersDocument(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's words.")
	calls := a.ai.calls.Load()
	if r := bob.post(docPath(id, "/rewrite"), map[string]any{"style": "brief"}); r.Status != http.StatusNotFound {
		t.Errorf("got %d", r.Status)
	}
	if a.ai.calls.Load() != calls {
		t.Error("Alice's text must never be sent to the model for Bob")
	}
}

// ---- suggested questions ----------------------------------------------------------------------------------

func TestSuggestedQuestionsAreMadeOnceAndKept(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "1")
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")

	first := cl.post(docPath(id, "/suggestions"), nil)
	if first.Status != http.StatusOK || len(first.JSON()["questions"].([]any)) != 4 {
		t.Fatalf("suggestions: %d %s", first.Status, first.Body)
	}
	second := cl.post(docPath(id, "/suggestions"), nil)
	if second.Status != http.StatusOK || string(second.Body) != string(first.Body) {
		t.Errorf("the same questions again: %s", second.Body)
	}
	if n := a.ai.suggestCalls.Load(); n != 1 {
		t.Errorf("the model wrote them once, not %d times", n)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(chats),0) FROM daily_usage"); n != 0 {
		t.Errorf("suggestions do not use the chat allowance, used %d", n)
	}
}

func TestSuggestionsFailCleanlyAndStayPrivate(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's words.")

	if r := bob.post(docPath(id, "/suggestions"), nil); r.Status != http.StatusNotFound {
		t.Errorf("another user's document: %d", r.Status)
	}
	a.ai.fail.Store(true)
	if r := alice.post(docPath(id, "/suggestions"), nil); r.Status < 500 {
		t.Errorf("a failing AI service: %d", r.Status)
	}
	a.ai.fail.Store(false)
	if r := alice.post(docPath(id, "/suggestions"), nil); r.Status != http.StatusOK {
		t.Errorf("and it recovers: %d", r.Status)
	}
}

// ---- flashcards and quiz ------------------------------------------------------------------------------------

func TestFlashcardsAndAQuizAreMadeAndReplayedForFree(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "2")
	cl, _ := a.newUser()
	id := saveText(t, cl, "Some pasted words.")

	cards := cl.post(docPath(id, "/study"), map[string]string{"kind": "flashcards"})
	if cards.Status != http.StatusOK || cards.JSON()["cached"] != false || len(cards.JSON()["cards"].([]any)) != 3 {
		t.Fatalf("flashcards: %d %s", cards.Status, cards.Body)
	}
	quiz := cl.post(docPath(id, "/study"), map[string]string{"kind": "quiz"})
	if quiz.Status != http.StatusOK || len(quiz.JSON()["questions"].([]any)) != 3 {
		t.Fatalf("quiz: %d %s", quiz.Status, quiz.Body)
	}
	// Both kinds are kept side by side, and showing them again is free even with the allowance used up.
	for _, kind := range []string{"flashcards", "quiz"} {
		again := cl.post(docPath(id, "/study"), map[string]string{"kind": kind})
		if again.Status != http.StatusOK || again.JSON()["cached"] != true {
			t.Errorf("%s replay: %d %s", kind, again.Status, again.Body)
		}
	}
	if n := a.ai.studyCalls.Load(); n != 2 {
		t.Errorf("two model calls, got %d", n)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(chats),0) FROM daily_usage"); n != 2 {
		t.Errorf("making each kind counts once, got %d", n)
	}
	if r := cl.post(docPath(id, "/study"), map[string]any{"kind": "quiz", "regenerate": true}); r.Status != http.StatusTooManyRequests {
		t.Errorf("a new quiz needs allowance: %d", r.Status)
	}
}

func TestStudyRequestsAreValidatedAndFailuresRefunded(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "5")
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's words.")

	if r := alice.post(docPath(id, "/study"), map[string]string{"kind": "poem"}); r.Status != http.StatusBadRequest {
		t.Errorf("an unknown kind: %d", r.Status)
	}
	if r := bob.post(docPath(id, "/study"), map[string]string{"kind": "quiz"}); r.Status != http.StatusNotFound {
		t.Errorf("another user's document: %d", r.Status)
	}
	a.ai.fail.Store(true)
	if r := alice.post(docPath(id, "/study"), map[string]string{"kind": "quiz"}); r.Status < 500 {
		t.Errorf("a failing AI service: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(chats),0) FROM daily_usage"); n != 0 {
		t.Errorf("failures are not charged, got %d", n)
	}
}

// ---- sharing -----------------------------------------------------------------------------------------------------

func TestASummaryCanBeSharedByALinkAndTheLinkCanBeTurnedOff(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	id := saveText(t, cl, "A secret source text that must stay private.")
	cl.put(docPath(id, ""), map[string]string{"filename": "Quarterly notes"})

	r := cl.post(docPath(id, "/share"), nil)
	token := r.Str("token")
	if r.Status != http.StatusOK || len(token) != 43 {
		t.Fatalf("share: %d %s", r.Status, r.Body)
	}
	if again := cl.post(docPath(id, "/share"), nil); again.Str("token") != token {
		t.Errorf("sharing twice gives the same link: %s", again.Body)
	}
	if got := listedDocuments(t, cl)[0]["shareToken"]; got != token {
		t.Errorf("the owner sees the link in the list: %v", got)
	}

	// Anyone can read it, with no account.
	anon := a.newClient()
	page := anon.get("/shared/" + token)
	if page.Status != http.StatusOK || page.JSON()["title"] != "Quarterly notes" || page.JSON()["summary"] == "" {
		t.Fatalf("shared page: %d %s", page.Status, page.Body)
	}
	for _, secret := range []string{"secret source text", email, "content", "user"} {
		if strings.Contains(strings.ToLower(string(page.Body)), strings.ToLower(secret)) {
			t.Errorf("the shared page must not reveal %q: %s", secret, page.Body)
		}
	}
	if page.Header.Get("Cache-Control") != "no-store" || !strings.Contains(page.Header.Get("X-Robots-Tag"), "noindex") {
		t.Errorf("not cached, not indexed: %v", page.Header)
	}

	if r := cl.delete(docPath(id, "/share"), nil); r.Status != http.StatusOK {
		t.Fatalf("unshare: %d", r.Status)
	}
	if page := anon.get("/shared/" + token); page.Status != http.StatusNotFound {
		t.Errorf("a turned-off link: %d", page.Status)
	}
	if got := listedDocuments(t, cl)[0]["shareToken"]; got != nil {
		t.Errorf("the list no longer shows a link: %v", got)
	}
	if again := cl.post(docPath(id, "/share"), nil); again.Str("token") == token {
		t.Error("a new link is a new secret")
	}
}

func TestSharedLinksStopWhenTheDocumentIsDeletedAndRefuseGuesses(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Words.")
	token := cl.post(docPath(id, "/share"), nil).Str("token")
	anon := a.newClient()

	if r := cl.delete(docPath(id, ""), nil); r.Status != http.StatusOK {
		t.Fatalf("delete: %d", r.Status)
	}
	if r := anon.get("/shared/" + token); r.Status != http.StatusNotFound {
		t.Errorf("a deleted document: %d", r.Status)
	}
	for _, guess := range []string{"short", strings.Repeat("a", 43), strings.Repeat("a", 44), "../etc/passwd", strings.Repeat("%", 43)} {
		if r := anon.get("/shared/" + url.PathEscape(guess)); r.Status != http.StatusNotFound {
			t.Errorf("%q: %d", guess, r.Status)
		}
	}
}

func TestOnlyTheOwnerCanShareOrUnshare(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's words.")
	token := alice.post(docPath(id, "/share"), nil).Str("token")

	if r := bob.post(docPath(id, "/share"), nil); r.Status != http.StatusNotFound {
		t.Errorf("Bob shares: %d", r.Status)
	}
	if r := bob.delete(docPath(id, "/share"), nil); r.Status != http.StatusNotFound {
		t.Errorf("Bob unshares: %d", r.Status)
	}
	if r := a.newClient().get("/shared/" + token); r.Status != http.StatusOK {
		t.Errorf("Alice's link still works: %d", r.Status)
	}
	if r := a.newClient().post(docPath(id, "/share"), nil); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
}

// ---- email ---------------------------------------------------------------------------------------------------------

func TestASummaryIsEmailedToItsOwnerOnly(t *testing.T) {
	a := newApp(t)
	alice, aliceEmail := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Words for the email.")
	alice.put(docPath(id, ""), map[string]string{"filename": "Lease notes"})
	a.mail.waitFor(t, 2) // the two sign-up confirmations

	r := alice.post(docPath(id, "/email"), map[string]string{"to": "attacker@evil.example"})
	if r.Status != http.StatusAccepted || !strings.Contains(r.Str("message"), aliceEmail) {
		t.Fatalf("email: %d %s", r.Status, r.Body)
	}
	sent := a.mail.waitFor(t, 3)[2]
	if sent.To != aliceEmail || sent.Subject != "Summary: Lease notes" || !strings.Contains(sent.Text, "fake summary") {
		t.Errorf("the mail: %+v", sent)
	}

	if r := bob.post(docPath(id, "/email"), nil); r.Status != http.StatusNotFound {
		t.Errorf("another user's document: %d", r.Status)
	}
	if len(a.mail.all()) != 3 {
		t.Errorf("nothing else was sent: %+v", a.mail.all())
	}
}

// ---- standing instructions ------------------------------------------------------------------------------------------

func TestStandingInstructionsAreSavedAndShownWithTheUser(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	r := cl.put("/account/instructions", map[string]string{"customInstructions": "  Focus on costs\n\nand deadlines.  "})
	if r.Status != http.StatusOK {
		t.Fatalf("save: %d %s", r.Status, r.Body)
	}
	me := cl.get("/auth/me").JSON()["user"].(map[string]any)
	if me["customInstructions"] != "Focus on costs and deadlines." {
		t.Errorf("me: %v", me)
	}
	cl.put("/account/instructions", map[string]string{"customInstructions": ""})
	if cl.get("/auth/me").JSON()["user"].(map[string]any)["customInstructions"] != "" {
		t.Error("they can be cleared")
	}
	if r := cl.put("/account/instructions", map[string]string{"customInstructions": strings.Repeat("x", 501)}); r.Status != http.StatusBadRequest {
		t.Errorf("too long: %d", r.Status)
	}
	if r := a.newClient().put("/account/instructions", map[string]string{"customInstructions": "x"}); r.Status != http.StatusUnauthorized {
		t.Errorf("signed out: %d", r.Status)
	}
}

func TestInstructionsReachEverySummaryOfTheirOwnerAndNobodyElses(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	other, _ := a.newUser()
	cl.put("/account/instructions", map[string]string{"customInstructions": "Mention deadlines"})

	sent := func() string { s, _ := a.ai.lastSummary.Load().(string); return s }
	textID := saveText(t, cl, "Pasted words.")
	if !strings.Contains(sent(), "instructions=Mention+deadlines") && !strings.Contains(sent(), "instructions=Mention%20deadlines") {
		t.Errorf("pasted text: %q", sent())
	}
	if r := cl.upload("/summarize", "file", map[string][]byte{"a.pdf": pdfBytes}); r.Status != http.StatusOK {
		t.Fatalf("pdf: %d", r.Status)
	}
	if !strings.Contains(sent(), "Mention deadlines") {
		t.Errorf("a PDF: %q", sent())
	}
	cl.post(docPath(textID, "/rewrite"), map[string]any{"style": "brief"})
	if !strings.Contains(sent(), "instructions=") {
		t.Errorf("a rewrite: %q", sent())
	}

	other.post("/summarize-text", map[string]string{"text": "Other words."})
	if strings.Contains(sent(), "instructions") {
		t.Errorf("another user's summaries carry nothing: %q", sent())
	}
	a.newClient().post("/public/summarize-text", map[string]string{"text": "Anonymous words."})
	if strings.Contains(sent(), "instructions") {
		t.Errorf("an anonymous summary carries nothing: %q", sent())
	}
}
