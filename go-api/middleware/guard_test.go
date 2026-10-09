package middleware

import (
	"regexp"
	"strings"
	"testing"
)

func TestTheLimitsHaveSensibleDefaultsAndCanBeChanged(t *testing.T) {
	t.Setenv("ANON_SUMMARIES_PER_DAY", "")
	t.Setenv("AI_REQUESTS_PER_DAY", "")
	if got := AnonymousLimit(); got != 10 {
		t.Errorf("anonymous default = %d, want 10", got)
	}
	if got := DailyBudgetLimit(); got != 5000 {
		t.Errorf("budget default = %d, want 5000", got)
	}

	t.Setenv("ANON_SUMMARIES_PER_DAY", "3")
	t.Setenv("AI_REQUESTS_PER_DAY", "250")
	if AnonymousLimit() != 3 || DailyBudgetLimit() != 250 {
		t.Errorf("set values are used: %d, %d", AnonymousLimit(), DailyBudgetLimit())
	}

	// 0 means unlimited; nonsense falls back to the default rather than switching the guard off
	t.Setenv("ANON_SUMMARIES_PER_DAY", "0")
	if AnonymousLimit() != 0 {
		t.Error("0 means unlimited")
	}
	for _, junk := range []string{"-5", "many", "1.5", " "} {
		t.Setenv("ANON_SUMMARIES_PER_DAY", junk)
		t.Setenv("AI_REQUESTS_PER_DAY", junk)
		if AnonymousLimit() != DefaultAnonymousPerDay || DailyBudgetLimit() != DefaultDailyBudget {
			t.Errorf("%q should fall back to the defaults, got %d and %d", junk, AnonymousLimit(), DailyBudgetLimit())
		}
	}
}

func TestAVisitorKeyIsAKeyedHashOfTheAddress(t *testing.T) {
	t.Setenv("IP_HASH_KEY", "one")
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	a := visitorKey("203.0.113.9")
	if !hex32.MatchString(a) || strings.Contains(a, "203") {
		t.Errorf("a short hash, not the address: %q", a)
	}
	if visitorKey("203.0.113.9") != a {
		t.Error("the same address gives the same key")
	}
	if visitorKey("203.0.113.10") == a {
		t.Error("different addresses give different keys")
	}
	if visitorKey("::ffff:203.0.113.9") != a {
		t.Error("an IPv4 address written the IPv6 way is the same visitor")
	}

	// the key depends on the secret, so the stored values cannot be turned into addresses by trying them all
	t.Setenv("IP_HASH_KEY", "two")
	if visitorKey("203.0.113.9") == a {
		t.Error("a different secret gives a different key")
	}
	t.Setenv("IP_HASH_KEY", "")
	t.Setenv("AI_SERVICE_TOKEN", "service-secret")
	fallback := visitorKey("203.0.113.9")
	t.Setenv("AI_SERVICE_TOKEN", "another-secret")
	if visitorKey("203.0.113.9") == fallback {
		t.Error("without IP_HASH_KEY the service token is the secret")
	}
}

func TestAnIPv6VisitorIsTheWhole64(t *testing.T) {
	a := visitorKey("2001:db8:1:2::1")
	if visitorKey("2001:db8:1:2:ffff:ffff:ffff:ffff") != a {
		t.Error("addresses within one /64 are one visitor")
	}
	if visitorKey("2001:db8:1:3::1") == a {
		t.Error("another /64 is another visitor")
	}
}

func TestSomethingThatIsNotAnAddressStillGetsAKey(t *testing.T) {
	if k := visitorKey("unix-socket"); len(k) != 32 {
		t.Errorf("key = %q", k)
	}
	if visitorKey("") == visitorKey("203.0.113.9") {
		t.Error("an empty address is not any visitor's")
	}
}
