package controllers

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	good := map[string]string{
		"  User@Example.com ": "user@example.com",
		"a.b+c@sub.domain.io": "a.b+c@sub.domain.io",
	}
	for in, want := range good {
		got, err := normalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("normalizeEmail(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	for _, in := range []string{"", "plain", "a@b", "Name <a@b.com>", "a@b.com, c@d.com", strings.Repeat("a", 250) + "@b.com"} {
		if _, err := normalizeEmail(in); err == nil {
			t.Errorf("normalizeEmail(%q) should fail", in)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if validatePassword("short") == nil {
		t.Error("short password should fail")
	}
	if validatePassword(strings.Repeat("x", 73)) == nil {
		t.Error("password over 72 bytes should fail")
	}
	if err := validatePassword("long-enough-1"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
}

func TestParseSummaryParams(t *testing.T) {
	opts, err := parseSummaryParams("", "", "", "")
	if err != nil || opts != (summaryOptions{Words: defaultWords, Style: defaultStyle, Language: defaultLanguage}) {
		t.Errorf("defaults = %+v, %v", opts, err)
	}
	opts, err = parseSummaryParams("300", "5", "bullets", "Serbian")
	if err != nil || opts != (summaryOptions{Words: 300, Pages: 5, Style: "bullets", Language: "Serbian"}) {
		t.Errorf("valid = %+v, %v", opts, err)
	}
	for _, c := range [][4]string{
		{"abc", "", "", ""}, {"10", "", "", ""}, {"5000", "", "", ""},
		{"", "-1", "", ""}, {"", "99", "", ""}, {"", "x", "", ""},
		{"", "", "poem", ""}, {"", "", "", "Klingon"},
	} {
		if _, err := parseSummaryParams(c[0], c[1], c[2], c[3]); err == nil {
			t.Errorf("parseSummaryParams%q should fail", c)
		}
	}
}
