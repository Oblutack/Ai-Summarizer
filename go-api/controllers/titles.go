package controllers

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxTitleRunes = 60
	// A title is cut at a word boundary, but not so early that a long first word leaves almost nothing.
	minTitleRunes = 20
)

// titleFromText names a pasted document after its first words, so a library full of pasted texts is
// not a list of identical "Pasted Text" rows. Text with nothing readable in it keeps the fallback.
func titleFromText(text, fallback string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimLeftFunc(line, func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune("#>*-_=+|`~", r)
		})
		line = strings.Join(strings.Fields(line), " ")
		if !strings.ContainsFunc(line, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		return shorten(line)
	}
	return fallback
}

// shorten cuts at the last word boundary within maxTitleRunes and adds an ellipsis.
func shorten(line string) string {
	if utf8.RuneCountInString(line) <= maxTitleRunes {
		return strings.TrimRight(line, " .,;:")
	}
	cut := []rune(line)[:maxTitleRunes]
	for i := len(cut) - 1; i >= minTitleRunes; i-- {
		if cut[i] == ' ' {
			cut = cut[:i]
			break
		}
	}
	return strings.TrimRight(string(cut), " .,;:!?-–—") + "…"
}
