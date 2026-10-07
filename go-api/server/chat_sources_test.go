package server

import (
	"net/http"
	"strconv"
	"testing"
)

// savedDocumentID summarizes some text as the client, so a saved document exists, and returns its id.
func savedDocumentID(t *testing.T, cl *client) int {
	t.Helper()
	if r := cl.post("/summarize-text", map[string]string{"text": "A document that can be chatted with."}); r.Status != 200 {
		t.Fatalf("summarize: %d %s", r.Status, r.Body)
	}
	var docs []map[string]any
	if err := jsonUnmarshal(cl.get("/documents").Body, &docs); err != nil || len(docs) == 0 {
		t.Fatalf("listing documents: %v", err)
	}
	return int(docs[0]["ID"].(float64))
}

func TestChatAnswersCarryTheirSources(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	id := savedDocumentID(t, cl)

	r := cl.post("/documents/"+strconv.Itoa(id)+"/chat", map[string]string{"question": "what?"})
	if r.Status != http.StatusOK {
		t.Fatalf("chat: %d %s", r.Status, r.Body)
	}
	body := r.JSON()
	if body["answer"] != "fake answer [1]" {
		t.Errorf("answer: %v", body["answer"])
	}
	sources, _ := body["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("want one source, got %v", body["sources"])
	}
	s := sources[0].(map[string]any)
	if s["id"] != float64(1) || s["text"] != "the cited passage" || s["page"] != float64(2) || s["pageEnd"] != float64(3) || s["document"] != "report.pdf" {
		t.Errorf("source was changed on the way through: %v", s)
	}
}
