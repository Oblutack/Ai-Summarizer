package controllers

import (
	"strings"
	"testing"
)

func TestSearchQueryIsAnOrQueryOfTheQuestionsWords(t *testing.T) {
	got := searchQuery("How much is the Rent, and when is it due? Rent!")
	// Words of four letters or more match by prefix; short ones stay exact; duplicates are dropped.
	for _, want := range []string{"much:*", "rent:*", "when:*", "how", "is", "the", "and", "it", "due"} {
		if !strings.Contains(" "+strings.ReplaceAll(got, " | ", " ")+" ", " "+want+" ") {
			t.Errorf("query %q is missing %q", got, want)
		}
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(got, " | ") {
		if seen[part] {
			t.Errorf("%q appears twice in %q", part, got)
		}
		seen[part] = true
	}
}

func TestSearchQueryCannotContainSearchSyntax(t *testing.T) {
	got := searchQuery(`bridges & !rivers | (x) <-> y:* ' ); DROP TABLE documents; --`)
	for _, part := range strings.Split(got, " | ") {
		word := strings.TrimSuffix(part, ":*")
		for _, r := range word {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				t.Fatalf("%q in %q is not a plain word", part, got)
			}
		}
	}
	if strings.ContainsAny(got, "&!()<>'\";-") {
		t.Errorf("syntax characters survived: %q", got)
	}
}

func TestSearchQueryKeepsOtherLanguagesAndCapsTheLength(t *testing.T) {
	if got := searchQuery("Kühlschrank kaputt"); got != "kühlschrank:* | kaputt:*" {
		t.Errorf("non-English words are kept whole: %q", got)
	}
	long := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron ", 2)
	if n := len(strings.Split(searchQuery(long), " | ")); n != maxQueryTerms {
		t.Errorf("at most %d terms, got %d", maxQueryTerms, n)
	}
}

func TestSearchQueryOfNothingSearchableIsEmpty(t *testing.T) {
	for _, q := range []string{"", "   ", "?!... ,", "a"} {
		if got := searchQuery(q); got != "" {
			t.Errorf("%q should have no searchable words, got %q", q, got)
		}
	}
}
