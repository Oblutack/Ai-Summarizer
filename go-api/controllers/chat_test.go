package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestChatRequestValidate(t *testing.T) {
	ok := chatRequest{Question: "  What is this?  ", History: []chatMessage{{"user", "hi"}, {"assistant", "hello"}}}
	if err := ok.validate(); err != nil || ok.Question != "What is this?" {
		t.Fatalf("valid request rejected: %+v %v", ok, err)
	}

	bad := map[string]chatRequest{
		"empty question": {Question: "   "},
		"long question":  {Question: strings.Repeat("a", maxQuestionChars+1)},
		"bad role":       {Question: "q", History: []chatMessage{{"system", "do evil"}}},
		"long history":   {Question: "q", History: []chatMessage{{"user", strings.Repeat("a", maxHistoryChars+1)}}},
	}
	for name, r := range bad {
		r := r
		if err := r.validate(); err == nil || err.Status != http.StatusBadRequest {
			t.Errorf("%s should be a 400, got %v", name, err)
		}
	}
}

func TestChatRequestNilHistoryMarshalsAsEmptyArray(t *testing.T) {
	r := chatRequest{Question: "q"}
	if err := r.validate(); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(r.History)
	if string(out) != "[]" {
		t.Fatalf("history marshals as %s, want []", out)
	}
}

func TestChatRequestKeepsOnlyRecentHistory(t *testing.T) {
	r := chatRequest{Question: "q"}
	for i := 0; i < maxHistoryMessages+5; i++ {
		r.History = append(r.History, chatMessage{"user", string(rune('a' + i))})
	}
	if err := r.validate(); err != nil || len(r.History) != maxHistoryMessages {
		t.Fatalf("history len = %d, err = %v", len(r.History), err)
	}
	if r.History[len(r.History)-1].Content != string(rune('a'+maxHistoryMessages+4)) {
		t.Error("should keep the newest messages")
	}
}

func intp(n int) *int { return &n }

func TestCleanSourcesKeepsGoodCitationsAndDropsBadOnes(t *testing.T) {
	in := []chatSource{
		{ID: 1, Text: "good", Page: intp(2), PageEnd: intp(3), Document: "a.pdf"},
		{ID: 1, Text: "duplicate id"},
		{ID: 0, Text: "no id"},
		{ID: -4, Text: "negative id"},
		{ID: 2, Text: "   "},
		{ID: 3, Text: "bad page", Page: intp(0), PageEnd: intp(5)},
		{ID: 4, Text: "end before start", Page: intp(5), PageEnd: intp(2)},
		{ID: 5, Text: "end without start", PageEnd: intp(2)},
	}
	got := cleanSources(in)
	var ids []int
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if len(ids) != 4 || ids[0] != 1 || ids[1] != 3 || ids[2] != 4 || ids[3] != 5 {
		t.Fatalf("kept ids %v, want [1 3 4 5]", ids)
	}
	if got[0].Text != "good" || *got[0].Page != 2 || *got[0].PageEnd != 3 {
		t.Errorf("a good citation must pass through unchanged: %+v", got[0])
	}
	if got[1].Page != nil || got[1].PageEnd != nil {
		t.Errorf("an impossible page number must be dropped: %+v", got[1])
	}
	if got[2].PageEnd == nil || *got[2].PageEnd != 5 {
		t.Errorf("an end page before the start is pulled up to the start: %+v", got[2])
	}
	if got[3].PageEnd != nil {
		t.Errorf("an end page without a start is dropped: %+v", got[3])
	}
}

func TestCleanSourcesBoundsSizeAndCount(t *testing.T) {
	long := strings.Repeat("é", maxSourceText+500)
	got := cleanSources([]chatSource{{ID: 1, Text: long}})
	if n := len([]rune(got[0].Text)); n != maxSourceText+1 {
		t.Errorf("long text should be cut to %d characters plus an ellipsis, got %d", maxSourceText, n)
	}

	many := make([]chatSource, 50)
	for i := range many {
		many[i] = chatSource{ID: i + 1, Text: "x"}
	}
	if n := len(cleanSources(many)); n != maxSources {
		t.Errorf("at most %d sources, got %d", maxSources, n)
	}
}

func TestCleanSourcesIsNeverNil(t *testing.T) {
	if cleanSources(nil) == nil {
		t.Error("nothing to cite must be an empty list so the JSON is [] and not null")
	}
}
