package server

import (
	"net/http"
	"strconv"
	"testing"
)

func podcastURL(id int) string { return "/documents/" + strconv.Itoa(id) + "/podcast" }

func chatsUsed(cl *client) float64 {
	return cl.get("/usage").JSON()["chats"].(map[string]any)["used"].(float64)
}

func TestAPodcastIsWrittenOnceAndStored(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "A document worth talking about.")

	before := a.ai.calls.Load()
	r := cl.post(podcastURL(id), map[string]any{})
	if r.Status != http.StatusOK {
		t.Fatalf("podcast: %d %s", r.Status, r.Body)
	}
	body := r.JSON()
	turns := body["turns"].([]any)
	if body["title"] != "Fake episode" || len(turns) != 4 || body["cached"] != false {
		t.Fatalf("unexpected script: %s", r.Body)
	}
	first := turns[0].(map[string]any)
	if first["speaker"] != "A" || first["text"] != "So what is this about?" {
		t.Errorf("first turn: %v", first)
	}
	if a.ai.calls.Load() != before+1 {
		t.Error("the script costs one model call")
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE podcast IS NOT NULL"); n != 1 {
		t.Errorf("the script is stored with the document, %d stored", n)
	}
}

func TestReplayingAPodcastCostsNothing(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "10")
	cl, _ := a.newUser()
	id := saveText(t, cl, "A document worth talking about.")
	cl.post(podcastURL(id), map[string]any{})
	used := chatsUsed(cl)
	calls := a.ai.calls.Load()

	r := cl.post(podcastURL(id), nil)
	if r.Status != 200 || r.JSON()["cached"] != true || len(r.JSON()["turns"].([]any)) != 4 {
		t.Fatalf("replay: %d %s", r.Status, r.Body)
	}
	if a.ai.calls.Load() != calls {
		t.Error("a stored script must not call the model again")
	}
	if chatsUsed(cl) != used {
		t.Error("a replay must not use the chat allowance")
	}
}

func TestRegeneratingOrChangingLanguageWritesANewScript(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "A document worth talking about.")
	cl.post(podcastURL(id), map[string]any{})
	calls := a.ai.calls.Load()

	if r := cl.post(podcastURL(id), map[string]any{"regenerate": true}); r.JSON()["cached"] != false {
		t.Errorf("regenerate must write again: %s", r.Body)
	}
	if r := cl.post(podcastURL(id), map[string]any{"language": "Spanish"}); r.JSON()["cached"] != false || r.JSON()["language"] != "Spanish" {
		t.Errorf("a different language needs its own script: %s", r.Body)
	}
	if a.ai.calls.Load() != calls+2 {
		t.Errorf("two new scripts, two model calls: %d", a.ai.calls.Load()-calls)
	}
	// And the Spanish one is now the stored one.
	if r := cl.post(podcastURL(id), map[string]any{"language": "Spanish"}); r.JSON()["cached"] != true {
		t.Errorf("the Spanish script should replay: %s", r.Body)
	}
}

func TestAPodcastUsesTheChatAllowanceWhenWritten(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "1")
	cl, _ := a.newUser()
	first := saveText(t, cl, "First document.")
	second := saveText(t, cl, "Second document.")

	if r := cl.post(podcastURL(first), map[string]any{}); r.Status != 200 {
		t.Fatalf("first: %d %s", r.Status, r.Body)
	}
	if r := cl.post(podcastURL(second), map[string]any{}); r.Status != http.StatusTooManyRequests {
		t.Errorf("a second script is over the allowance: %d", r.Status)
	}
	if r := cl.post(podcastURL(first), map[string]any{}); r.Status != 200 {
		t.Errorf("but replaying the first is still free: %d", r.Status)
	}
}

func TestPodcastsAreOnlyForTheDocumentsOwner(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := saveText(t, alice, "Alice's document.")

	if r := bob.post(podcastURL(id), map[string]any{}); r.Status != http.StatusNotFound {
		t.Errorf("Bob must not make a podcast of Alice's document: %d", r.Status)
	}
	if r := a.newClient().post(podcastURL(id), map[string]any{}); r.Status != http.StatusUnauthorized {
		t.Errorf("a visitor must sign in: %d", r.Status)
	}
	if r := alice.post("/documents/abc/podcast", map[string]any{}); r.Status != http.StatusBadRequest {
		t.Errorf("bad id: %d", r.Status)
	}
}

func TestAFailingModelGivesACleanErrorAndKeepsTheAllowance(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_CHAT_PER_DAY", "5")
	cl, _ := a.newUser()
	id := saveText(t, cl, "A document worth talking about.")

	a.ai.fail.Store(true)
	r := cl.post(podcastURL(id), map[string]any{})
	if r.Status != http.StatusBadGateway || r.Str("error") == "" {
		t.Errorf("a failing AI service gives a clear 502: %d %s", r.Status, r.Body)
	}
	if chatsUsed(cl) != 0 {
		t.Error("a failed script must not use the allowance")
	}
	if n := count(t, a.db, "SELECT count(*) FROM documents WHERE podcast IS NOT NULL"); n != 0 {
		t.Error("nothing is stored for a failed script")
	}
}

func TestDocumentsWithoutASummaryCannotBecomeAPodcast(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := saveText(t, cl, "A document worth talking about.")
	a.db.Exec("UPDATE documents SET summary = '' WHERE id = ?", id)
	if r := cl.post(podcastURL(id), map[string]any{}); r.Status != http.StatusConflict {
		t.Errorf("nothing to talk about: %d %s", r.Status, r.Body)
	}
}
