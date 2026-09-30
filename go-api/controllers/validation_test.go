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
	if w, p, err := parseSummaryParams("", ""); err != nil || w != defaultWords || p != 0 {
		t.Errorf("defaults = %d, %d, %v", w, p, err)
	}
	if w, p, err := parseSummaryParams("300", "5"); err != nil || w != 300 || p != 5 {
		t.Errorf("valid = %d, %d, %v", w, p, err)
	}
	for _, c := range [][2]string{{"abc", ""}, {"10", ""}, {"5000", ""}, {"", "-1"}, {"", "99"}, {"", "x"}} {
		if _, _, err := parseSummaryParams(c[0], c[1]); err == nil {
			t.Errorf("parseSummaryParams(%q, %q) should fail", c[0], c[1])
		}
	}
}
