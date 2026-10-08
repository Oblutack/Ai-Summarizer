package controllers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTitleFromText(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"short first line", "Rental agreement.\nThe tenant pays", "Rental agreement"},
		{"leading blank lines and spaces", "\n\n   Q3   product \t update\nmore", "Q3 product update"},
		{"markdown marks are dropped", "## Meeting notes\n- one", "Meeting notes"},
		{"quote marks are dropped", "> Quoted opening line", "Quoted opening line"},
		{"decoration lines are skipped", "-----\n=====\nReal first line", "Real first line"},
		{"other scripts work", "日本語のテキスト、これは例です。", "日本語のテキスト、これは例です。"},
		{"a closing question mark stays", "Is this covered?", "Is this covered?"},
		{"only symbols falls back", "---\n***\n  \n", "Pasted Text"},
		{"empty falls back", "", "Pasted Text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := titleFromText(c.text, "Pasted Text"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestLongTitlesAreCutAtAWordBoundary(t *testing.T) {
	got := titleFromText("Our mobile app reached 480,000 monthly active users, up 18% from last quarter, driven by offline mode.", "x")
	if got != "Our mobile app reached 480,000 monthly active users, up 18%…" {
		t.Errorf("got %q", got)
	}
	if n := utf8.RuneCountInString(got); n > maxTitleRunes+1 {
		t.Errorf("title has %d characters", n)
	}
}

func TestAnUnbrokenLongWordIsCutHard(t *testing.T) {
	got := titleFromText(strings.Repeat("a", 200), "x")
	if utf8.RuneCountInString(got) != maxTitleRunes+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("got %q", got)
	}
}

func TestMultibyteTitlesAreNotCutMidCharacter(t *testing.T) {
	got := titleFromText(strings.Repeat("é", 100), "x")
	if !utf8.ValidString(got) {
		t.Errorf("invalid UTF-8: %q", got)
	}
}
