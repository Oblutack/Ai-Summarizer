package server

import (
	"net/http"
	"strconv"
	"testing"
)

func TestSummaryProofIsReturnedForTheOwner(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := savedDocumentID(t, cl)

	before := a.ai.calls.Load()
	r := cl.post("/documents/"+strconv.Itoa(id)+"/proof", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("proof: %d %s", r.Status, r.Body)
	}
	body := r.JSON()
	sentences := body["sentences"].([]any)
	if len(sentences) != 2 || body["found"] != float64(1) || body["verifiable"] != true {
		t.Fatalf("unexpected result: %s", r.Body)
	}
	claim := sentences[1].(map[string]any)
	passage := claim["passages"].([]any)[0].(map[string]any)
	if claim["support"] != "strong" || passage["page"] != float64(2) || passage["text"] != "The backing passage." {
		t.Errorf("the claim lost detail on the way through: %v", claim)
	}
	if sentences[0].(map[string]any)["support"] != nil {
		t.Error("headings carry no verdict")
	}
	if a.ai.calls.Load() != before+1 {
		t.Error("the AI service should be called exactly once")
	}
}

func TestProofDoesNotUseUpTheDailyQuota(t *testing.T) {
	a := newApp(t)
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "1")
	t.Setenv("QUOTA_CHAT_PER_DAY", "1")
	cl, _ := a.newUser()
	id := savedDocumentID(t, cl)
	for i := 0; i < 4; i++ {
		if r := cl.post("/documents/"+strconv.Itoa(id)+"/proof", nil); r.Status != http.StatusOK {
			t.Fatalf("check %d: %d %s", i+1, r.Status, r.Body)
		}
	}
	usage := cl.get("/usage").JSON()
	if usage["chats"].(map[string]any)["used"] != float64(0) {
		t.Errorf("proof must not count as chat: %v", usage)
	}
}

func TestProofIsOnlyForTheDocumentsOwner(t *testing.T) {
	a := newApp(t)
	alice, _ := a.newUser()
	bob, _ := a.newUser()
	id := strconv.Itoa(savedDocumentID(t, alice))

	if r := bob.post("/documents/"+id+"/proof", nil); r.Status != http.StatusNotFound {
		t.Errorf("Bob must not check Alice's document: %d", r.Status)
	}
	if r := a.newClient().post("/documents/"+id+"/proof", nil); r.Status != http.StatusUnauthorized {
		t.Errorf("a visitor must sign in: %d", r.Status)
	}
	if r := alice.post("/documents/abc/proof", nil); r.Status != http.StatusBadRequest {
		t.Errorf("bad id: %d", r.Status)
	}
	if r := alice.post("/documents/999999/proof", nil); r.Status != http.StatusNotFound {
		t.Errorf("unknown id: %d", r.Status)
	}
}

func TestProofNeedsTheStoredText(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := savedDocumentID(t, cl)
	a.db.Exec("UPDATE documents SET has_content = false, content = '' WHERE id = ?", id)
	r := cl.post("/documents/"+strconv.Itoa(id)+"/proof", nil)
	if r.Status != http.StatusConflict {
		t.Errorf("documents saved before the text was kept cannot be checked: %d %s", r.Status, r.Body)
	}
}

func TestProofFailuresAreReportedCleanly(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := savedDocumentID(t, cl)
	a.ai.fail.Store(true)
	r := cl.post("/documents/"+strconv.Itoa(id)+"/proof", nil)
	if r.Status != http.StatusBadGateway || r.Str("error") == "" {
		t.Errorf("a failing AI service should give a clear 502: %d %s", r.Status, r.Body)
	}
}
