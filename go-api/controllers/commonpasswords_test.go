package controllers

import "testing"

func TestTheCommonPasswordListIsLongAndAllEightCharactersOrMore(t *testing.T) {
	if len(commonPasswords) < 150 {
		t.Errorf("only %d common passwords listed", len(commonPasswords))
	}
	for word := range commonPasswords {
		if len(word) < minPasswordLen {
			t.Errorf("%q is shorter than the shortest allowed password, so listing it does nothing", word)
		}
		if word != lower(word) {
			t.Errorf("%q must be in lower case: the check ignores case", word)
		}
	}
}

func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func TestPatternsAreRecognizedAsTooSimple(t *testing.T) {
	simple := []string{
		"aaaaaaaa", "11111111", "abababab", "12341234", "abcabcabc", "xyxyxyxy", "abcdefgh", "87654321", "klmnopqr",
		"zyxwvuts", "asdfghjk", "qwertyui", "mnbvcxz1"[:7], "01234567", "ABCDEFGH",
	}
	for _, p := range simple {
		if !isTooSimple(p) {
			t.Errorf("%q should be too simple", p)
		}
	}
	fine := []string{"correct-horse", "Tr0ub4dor&3", "abcdefgx", "12345679", "qwertyuz", "a1b2c3d5", "hello world"}
	for _, p := range fine {
		if isTooSimple(p) {
			t.Errorf("%q should not be too simple", p)
		}
	}
	if !isTooSimple("") {
		t.Error("nothing is too simple")
	}
}

func TestTheEmailNameInAPasswordIsFoundButShortNamesAreNot(t *testing.T) {
	cases := []struct {
		password, email string
		want            bool
	}{
		{"alexandra2024", "alexandra@example.com", true},
		{"My-ALEXANDRA-pass", "alexandra@example.com", true},
		{"correct-horse-9", "alexandra@example.com", false},
		{"Journey-to-9-lakes", "jo@example.com", false}, // two letters: too short to mean anything
		{"anything", "", false},
		{"anything", "no-at-sign", false},
	}
	for _, c := range cases {
		if got := containsLocalPart(c.password, c.email); got != c.want {
			t.Errorf("containsLocalPart(%q, %q) = %v, want %v", c.password, c.email, got, c.want)
		}
	}
}

func TestValidatePasswordPutsTheRulesTogether(t *testing.T) {
	if err := validatePassword("Password123", "x@example.com"); err == nil {
		t.Error("a common password in another case is refused")
	}
	if err := validatePassword("alexandra-9-9", "alexandra@example.com"); err == nil {
		t.Error("the email name is refused")
	}
	if err := validatePassword("correct-horse-battery", "alexandra@example.com"); err != nil {
		t.Errorf("a good password: %v", err)
	}
}
