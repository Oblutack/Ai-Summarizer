package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// saveText saves pasted text as a document of the client's and returns its id.
func saveText(t *testing.T, cl *client, text string) int {
	t.Helper()
	if r := cl.post("/summarize-text", map[string]string{"text": text}); r.Status != 200 {
		t.Fatalf("summarize: %d %s", r.Status, r.Body)
	}
	return int(listedDocuments(t, cl)[0]["ID"].(float64))
}

func askLibrary(cl *client, question string) reply {
	return cl.post("/library/ask", map[string]string{"question": question})
}

// sentPassages returns the passages the gateway sent to the AI service for the last /ask.
func sentPassages(t *testing.T, a *app) []map[string]any {
	t.Helper()
	raw, _ := a.ai.lastAsk.Load().([]byte)
	var in struct {
		Passages []map[string]any `json:"passages"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("no /ask request was recorded: %v", err)
	}
	return in.Passages
}

func TestAQuestionIsAnsweredFromTheRightDocumentAcrossTheLibrary(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	apples := saveText(t, cl, "Apples are crisp and grown in orchards across the valley.")
	bananas := saveText(t, cl, "Bananas contain potassium and ripen quickly in warm kitchens.")
	saveText(t, cl, "The lease ends on the first of March and rent is due monthly.")

	r := askLibrary(cl, "How much potassium do bananas have?")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	passages := sentPassages(t, a)
	if len(passages) != 1 || passages[0]["text"] != "Bananas contain potassium and ripen quickly in warm kitchens." {
		t.Fatalf("only the banana passage should be sent to the model: %v", passages)
	}
	body := r.JSON()
	sources := body["sources"].([]any)
	if body["answer"] != "library answer [1]" || len(sources) != 1 {
		t.Fatalf("unexpected reply: %s", r.Body)
	}
	s := sources[0].(map[string]any)
	if s["documentId"] != float64(bananas) || s["documentId"] == float64(apples) || s["id"] != float64(1) {
		t.Errorf("the source must say which document it came from: %v", s)
	}
	if s["text"] != "Bananas contain potassium and ripen quickly in warm kitchens." {
		t.Errorf("source text: %v", s["text"])
	}
}

func TestDocumentsAreIndexedWhenSaved(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, "First paragraph of the document.\n\nSecond paragraph of the document.")
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 2 {
		t.Errorf("a document is cut into passages when it is saved, found %d", n)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE indexed_at IS NOT NULL"); n != 1 {
		t.Errorf("the document is marked indexed, found %d", n)
	}
}

func TestOlderDocumentsAreIndexedOnFirstUse(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	a.db.Exec("INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content) SELECT now(), now(), 'old.pdf', 's', id, true, 'Turbines convert wind into electricity.' FROM users")
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 0 {
		t.Fatalf("setup: %d", n)
	}

	r := askLibrary(cl, "What do turbines convert?")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 1 {
		t.Fatalf("an unindexed document must be found too: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 1 {
		t.Errorf("it should now be indexed, found %d passages", n)
	}
}

func TestOneUserNeverSearchesAnothersDocuments(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	saveText(t, alice, "Alice keeps her confidential merger plans for the zeppelin acquisition here.")

	r := askLibrary(bob, "What are the zeppelin acquisition plans?")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d", r.Status)
	}
	if got := r.JSON()["sources"].([]any); len(got) != 0 {
		t.Errorf("Bob must find nothing of Alice's: %v", got)
	}
	if a.ai.lastAsk.Load() != nil {
		t.Error("no passages means the model is never called, let alone with someone else's text")
	}
}

func TestNothingFoundMeansNoModelCallAndNoQuotaUsed(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "5")
	cl, _ := a.newUser()
	saveText(t, cl, "A document about gardening and compost.")

	r := askLibrary(cl, "Explain quantum chromodynamics")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 0 {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	if r.JSON()["answer"] == "" {
		t.Error("the user is told nothing was found")
	}
	if a.ai.lastAsk.Load() != nil {
		t.Error("the model must not be called without passages")
	}
	if used := cl.get("/usage").JSON()["chats"].(map[string]any)["used"]; used != float64(0) {
		t.Errorf("a question that made no model call must not use the allowance: %v", used)
	}
}

func TestAnsweredQuestionsUseTheChatAllowance(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "1")
	cl, _ := a.newUser()
	saveText(t, cl, "Solar panels convert sunlight into electricity on rooftops.")

	if r := askLibrary(cl, "How do solar panels work?"); r.Status != 200 {
		t.Fatalf("first: %d %s", r.Status, r.Body)
	}
	if r := askLibrary(cl, "How do solar panels work?"); r.Status != http.StatusTooManyRequests {
		t.Errorf("the second question is over the allowance: %d", r.Status)
	}
}

func TestDeletedDocumentsLeaveTheLibrary(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Seagulls nest on the harbour cliffs every spring.")
	if r := askLibrary(cl, "Where do seagulls nest?"); len(r.JSON()["sources"].([]any)) != 1 {
		t.Fatalf("setup: %s", r.Body)
	}

	if r := cl.delete("/documents/"+strconv.Itoa(id), nil); r.Status != 200 {
		t.Fatalf("delete: %d", r.Status)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 0 {
		t.Errorf("a deleted document's passages must go too, %d left", n)
	}
	a.ai.lastAsk.Store([]byte(nil))
	if got := askLibrary(cl, "Where do seagulls nest?").JSON()["sources"].([]any); len(got) != 0 {
		t.Errorf("a deleted document must not be found: %v", got)
	}
}

func TestDeletingAnAccountRemovesItsPassages(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	saveText(t, cl, "Some text that will be indexed and then erased.")
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 1 {
		t.Fatalf("setup: %d", n)
	}
	if r := cl.delete("/account", map[string]string{"confirmEmail": email, "password": goodPass}); r.Status != 200 {
		t.Fatalf("delete account: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM document_passages"); n != 0 {
		t.Errorf("no passage may outlive its account, %d left", n)
	}
}

func TestNoSingleDocumentCrowdsOutTheOthers(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	long := ""
	for i := 0; i < 10; i++ {
		long += "Harvest notes about wheat number " + strconv.Itoa(i) + ".\n\n"
	}
	saveText(t, cl, long)
	saveText(t, cl, "A different document that also mentions the wheat harvest.")

	askLibrary(cl, "Tell me about the wheat harvest")
	passages := sentPassages(t, a)
	if len(passages) > 5 {
		t.Errorf("at most %d passages per document, got %d in total", 4, len(passages))
	}
	if len(passages) < 2 {
		t.Errorf("both documents should be represented: %v", passages)
	}
}

func TestPDFSourcesCarryTheFileSoThePageCanBeOpened(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	// The fake AI turns the PDF into text through /summarize; give the document searchable content.
	if r := cl.upload("/summarize", "file", map[string][]byte{"manual.pdf": pdfBytes}); r.Status != 200 {
		t.Fatalf("upload: %d", r.Status)
	}
	a.db.Exec("UPDATE documents SET has_content = true, content = 'The warranty lasts seven years.', indexed_at = NULL")

	r := askLibrary(cl, "How long is the warranty?")
	sources := r.JSON()["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("expected a source: %s", r.Body)
	}
	s := sources[0].(map[string]any)
	if s["fileId"] == nil || s["fileName"] != "manual.pdf" {
		t.Errorf("a source from a document with a stored PDF names the file: %v", s)
	}
}

func TestLibraryQuestionsAreValidatedAndNeedASession(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	if r := askLibrary(cl, "   "); r.Status != http.StatusBadRequest {
		t.Errorf("blank question: %d", r.Status)
	}
	if r := askLibrary(cl, string(make([]byte, 0))); r.Status != http.StatusBadRequest {
		t.Errorf("empty question: %d", r.Status)
	}
	if r := a.newClient().post("/library/ask", map[string]string{"question": "hello there"}); r.Status != http.StatusUnauthorized {
		t.Errorf("visitors must sign in: %d", r.Status)
	}
	if r := askLibrary(cl, "?!... ,"); r.Status != http.StatusOK || r.JSON()["answer"] == "" {
		t.Errorf("a question with no searchable words gets a gentle answer: %d %s", r.Status, r.Body)
	}
}

func TestQuestionTextCannotFormSearchSyntax(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, "Plain document about bridges and rivers.")
	for _, q := range []string{"bridges & !rivers", "bridges' ); DROP TABLE documents; --", "(bridges | ", "bridges:*", "\\"} {
		if r := askLibrary(cl, q); r.Status != http.StatusOK {
			t.Errorf("%q must not break the search: %d %s", q, r.Status, r.Body)
		}
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents"); n != 1 {
		t.Error("the documents table must be untouched")
	}
}

func TestPassagesInOtherLanguagesAreFoundByPlainWordMatch(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, "Der Kühlschrank ist kaputt und muss repariert werden.")
	r := askLibrary(cl, "Kühlschrank kaputt")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 1 {
		t.Errorf("a non-English document is still searchable by its words: %d %s", r.Status, r.Body)
	}
}

func TestAWordFindsLongerWordsThatBeginWithIt(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, "Rental agreement. The tenant pays 950 euros per month, due on the first.")

	// The document never says "rent": it says "rental".
	r := askLibrary(cl, "How much is the rent?")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 1 {
		t.Errorf("rent should find rental: %d %s", r.Status, r.Body)
	}
}
