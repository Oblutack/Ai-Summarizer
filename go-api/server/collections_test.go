package server

import (
	"net/http"
	"testing"
)

// A collection is the group of documents that share a tag. A question can be asked of just that group.

func askCollection(cl *client, question, tag string) reply {
	return cl.post("/library/ask", map[string]string{"question": question, "tag": tag})
}

func tagDocument(t *testing.T, cl *client, id int, tags ...string) {
	t.Helper()
	if r := cl.put(docPath(id, ""), map[string]any{"tags": tags}); r.Status != http.StatusOK {
		t.Fatalf("tagging: %d %s", r.Status, r.Body)
	}
}

func sentTexts(t *testing.T, a *app) []string {
	t.Helper()
	var texts []string
	for _, p := range sentPassages(t, a) {
		texts = append(texts, p["text"].(string))
	}
	return texts
}

func TestAQuestionCanBeAskedOfOneCollection(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	garden := saveText(t, cl, "The garden lease allows keeping a cat but no dogs on the premises.")
	flat := saveText(t, cl, "The flat lease forbids every kind of pet including a cat or a dog.")
	saveText(t, cl, "Cats and dogs are popular pets in many households around the country.")
	tagDocument(t, cl, garden, "garden")
	tagDocument(t, cl, flat, "flat")

	// Everything: all three documents are searched.
	if r := askLibrary(cl, "Can I keep a cat?"); r.Status != http.StatusOK {
		t.Fatalf("ask all: %d %s", r.Status, r.Body)
	}
	if got := sentTexts(t, a); len(got) != 3 {
		t.Fatalf("without a collection every document is searched: %v", got)
	}

	// One collection: only its document is searched, and only it can be cited.
	r := askCollection(cl, "Can I keep a cat?", "flat")
	if r.Status != http.StatusOK {
		t.Fatalf("ask collection: %d %s", r.Status, r.Body)
	}
	got := sentTexts(t, a)
	if len(got) != 1 || got[0] != "The flat lease forbids every kind of pet including a cat or a dog." {
		t.Fatalf("only the flat document belongs to the collection: %v", got)
	}
	sources := r.JSON()["sources"].([]any)
	if len(sources) != 1 || sources[0].(map[string]any)["documentId"] != float64(flat) {
		t.Errorf("the answer may only cite the collection's documents: %s", r.Body)
	}
}

func TestACollectionNameIsTreatedLikeATag(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "Turbines convert wind into electricity for the whole valley.")
	tagDocument(t, cl, id, "Energy Course")

	// Tags are stored cleaned up (lower case, single spaces), and so is the collection that is asked.
	if r := askCollection(cl, "What do turbines convert?", "  ENERGY   course "); r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 1 {
		t.Fatalf("a differently typed name must still find the collection: %d %s", r.Status, r.Body)
	}
}

func TestAnEmptyOrUnknownCollectionIsAnsweredWithoutAModelCall(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "2")
	cl, _ := a.newUser()
	saveText(t, cl, "Turbines convert wind into electricity for the whole valley.")
	before := a.ai.calls.Load() // the summary made above

	for i := 0; i < 4; i++ {
		r := askCollection(cl, "What do turbines convert?", "no-such-collection")
		if r.Status != http.StatusOK || r.Str("answer") != "None of your documents has that tag, or they have no text to search." {
			t.Fatalf("unknown collection: %d %s", r.Status, r.Body)
		}
	}
	if n := a.ai.calls.Load() - before; n != 0 {
		t.Errorf("no document means no model call, the AI service was called %d times", n)
	}
	if n := count(t, a.db, "SELECT COALESCE(SUM(chats),0) FROM daily_usage"); n != 0 {
		t.Errorf("an answer without a model call must not use the allowance, usage = %d", n)
	}
}

func TestACollectionOnlyEverContainsTheUsersOwnDocuments(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	secret := saveText(t, alice, "Alice's secret: the vault code is written on the blue folder.")
	tagDocument(t, alice, secret, "notes")
	mine := saveText(t, bob, "Bob keeps notes about the vault in his own folder at home.")
	tagDocument(t, bob, mine, "notes")

	// The same collection name means each person's own documents, never a mix.
	r := askCollection(bob, "What is written about the vault folder?", "notes")
	if r.Status != http.StatusOK {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	got := sentTexts(t, a)
	if len(got) != 1 || got[0] != "Bob keeps notes about the vault in his own folder at home." {
		t.Fatalf("Bob's collection holds only Bob's documents: %v", got)
	}
	if id := r.JSON()["sources"].([]any)[0].(map[string]any)["documentId"]; id != float64(mine) {
		t.Errorf("the cited document must be Bob's: %v", id)
	}
}

func TestBadCollectionNamesAreRejected(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	saveText(t, cl, "Some words that could be searched.")
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'x'
	}
	if r := askCollection(cl, "Anything here?", string(long)); r.Status != http.StatusBadRequest {
		t.Errorf("a very long name: %d %s", r.Status, r.Body)
	}
}

func TestAskingACollectionOnlyIndexesThatCollectionsOlderDocuments(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	// Two documents saved before indexing existed: one in the collection, one outside it.
	a.db.Exec(`INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content, tags)
		SELECT now(), now(), 'inside.pdf', 's', id, true, 'Turbines convert wind into electricity.', '["energy"]'::jsonb FROM users`)
	a.db.Exec(`INSERT INTO documents (created_at, updated_at, filename, summary, user_id, has_content, content, tags)
		SELECT now(), now(), 'outside.pdf', 's', id, true, 'Turbines also appear in this unrelated brochure.', '[]'::jsonb FROM users`)

	r := askCollection(cl, "What do turbines convert?", "energy")
	if r.Status != http.StatusOK || len(r.JSON()["sources"].([]any)) != 1 {
		t.Fatalf("ask: %d %s", r.Status, r.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE filename = 'inside.pdf' AND indexed_at IS NOT NULL"); n != 1 {
		t.Errorf("the collection's document should now be indexed")
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE filename = 'outside.pdf' AND indexed_at IS NOT NULL"); n != 0 {
		t.Errorf("a question about one collection must not index documents outside it")
	}
}
