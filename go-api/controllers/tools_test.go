package controllers

import (
	"reflect"
	"strings"
	"testing"
)

func TestTagsAreNormalized(t *testing.T) {
	got, err := normalizeTags([]string{" Legal ", "LEGAL", "Housing   costs", "", "  "})
	if err != nil || !reflect.DeepEqual(got, []string{"legal", "housing costs"}) {
		t.Errorf("got %v, %v", got, err)
	}
	if got, _ := normalizeTags(nil); got == nil || len(got) != 0 {
		t.Errorf("no tags is an empty list, not nil: %#v", got)
	}
	if _, err := normalizeTags([]string{strings.Repeat("é", 31)}); err == nil {
		t.Error("a tag of 31 characters is too long (characters, not bytes)")
	}
	if _, err := normalizeTags([]string{strings.Repeat("é", 30)}); err != nil {
		t.Errorf("30 characters is fine: %v", err)
	}
}

func TestCleanLineFlattensWhitespaceAndDropsControlCharacters(t *testing.T) {
	if got := cleanLine("  a\tb\n\nc\u0000d\u0007 e  "); got != "a b cd e" {
		t.Errorf("got %q", got)
	}
}

func TestLikePatternsMatchLiterally(t *testing.T) {
	if got := likePattern(`50%_off\`); got != `%50\%\_off\\%` {
		t.Errorf("got %q", got)
	}
}

func TestSuggestionsAreCleanedAndBounded(t *testing.T) {
	in := []string{" What is it?\n", "what is it?", "", strings.Repeat("x", 141), "Who? ", "When?", "Why?", "How?", "Which?"}
	got := cleanSuggestions(in)
	if !reflect.DeepEqual(got, []string{"What is it?", "Who?", "When?", "Why?", "How?"}) {
		t.Errorf("got %v", got)
	}
}

func TestCardsAreCleanedAndBounded(t *testing.T) {
	var in []studyCard
	in = append(in, studyCard{"  Front  ", "Back"}, studyCard{"front", "Repeated"}, studyCard{"", "No front"}, studyCard{"No back", " "})
	for i := 0; i < 20; i++ {
		in = append(in, studyCard{Front: "Q" + strings.Repeat("x", i+1), Back: "A"})
	}
	got := cleanCards(in)
	if len(got) != maxCards || got[0] != (studyCard{"Front", "Back"}) {
		t.Errorf("got %d cards, first %v", len(got), got[0])
	}
}

func TestQuizQuestionsMustBeWellFormed(t *testing.T) {
	good := quizQuestion{Question: " One? ", Options: []string{"a", "b", "c", "d"}, Answer: 2, Explanation: " Because. "}
	cases := map[string]quizQuestion{
		"three options":       {Question: "Q?", Options: []string{"a", "b", "c"}, Answer: 0},
		"five options":        {Question: "Q?", Options: []string{"a", "b", "c", "d", "e"}, Answer: 0},
		"answer out of range": {Question: "Q?", Options: []string{"a", "b", "c", "d"}, Answer: 4},
		"negative answer":     {Question: "Q?", Options: []string{"a", "b", "c", "d"}, Answer: -1},
		"repeated option":     {Question: "Q?", Options: []string{"a", "A", "c", "d"}, Answer: 0},
		"empty option":        {Question: "Q?", Options: []string{"a", "", "c", "d"}, Answer: 0},
		"no question":         {Question: "  ", Options: []string{"a", "b", "c", "d"}, Answer: 0},
	}
	in := []quizQuestion{good, {Question: "one?", Options: []string{"w", "x", "y", "z"}, Answer: 0}}
	for _, q := range cases {
		in = append(in, q)
	}
	got := cleanQuiz(in)
	if len(got) != 1 || got[0].Question != "One?" || got[0].Explanation != "Because." || got[0].Answer != 2 {
		t.Errorf("only the good, first question survives: %+v", got)
	}
}

func TestShareTokensAreUnguessableAndRecognised(t *testing.T) {
	a, err := newShareToken()
	b, _ := newShareToken()
	if err != nil || a == b || !shareTokenPattern.MatchString(a) {
		t.Errorf("tokens: %q %q %v", a, b, err)
	}
	for _, bad := range []string{"", "short", a + "x", strings.Replace(a, a[:1], "=", 1), a[:42] + "/"} {
		if shareTokenPattern.MatchString(bad) {
			t.Errorf("%q must not be accepted", bad)
		}
	}
}
