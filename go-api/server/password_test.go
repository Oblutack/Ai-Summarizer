package server

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-summarizer/go-api/auth"

	"golang.org/x/crypto/bcrypt"
)

// Signing up must not tell anyone which addresses already have an account, and passwords are held to a standard:
// not a common one, not an obvious pattern, hashed with real work (which is raised for old hashes at the next login).

func withPasswordCost(t *testing.T, cost int) {
	t.Helper()
	old := auth.PasswordCost
	auth.PasswordCost = cost
	t.Cleanup(func() { auth.PasswordCost = old })
}

func hashOf(t *testing.T, a *app, email string) string {
	t.Helper()
	return a.userByEmail(email).Password
}

// ---- signup does not reveal who has an account -----------------------------------------------------

func TestSignupSaysTheSameWhetherOrNotTheAddressHasAnAccount(t *testing.T) {
	a := newApp(t)
	taken := uniqueEmail()
	a.signup(taken)
	a.mail.waitFor(t, 1) // the confirmation email of the first signup

	fresh := a.newClient().post("/signup", map[string]string{"email": uniqueEmail(), "password": goodPass})
	again := a.newClient().post("/signup", map[string]string{"email": taken, "password": goodPass})
	if fresh.Status != http.StatusOK || again.Status != fresh.Status {
		t.Fatalf("a new address got %d, a taken one %d: they must be the same", fresh.Status, again.Status)
	}
	if string(fresh.Body) != string(again.Body) {
		t.Errorf("the replies differ:\n new:   %s\n taken: %s", fresh.Body, again.Body)
	}
	if n := count(t, a.db, "SELECT count(*) FROM users WHERE lower(email) = ?", taken); n != 1 {
		t.Errorf("no second account may be made, found %d", n)
	}
}

func TestTheOwnerOfATakenAddressIsToldByEmailNotTheVisitor(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	a.mail.waitFor(t, 1)

	if r := a.newClient().post("/signup", map[string]string{"email": strings.ToUpper(email), "password": "Another-pass-77"}); r.Status != http.StatusOK {
		t.Fatalf("signup with a taken address: %d %s", r.Status, r.Body)
	}
	mails := a.mail.waitFor(t, 2)
	notice := mails[1]
	if notice.To != email || notice.Subject != "You already have an Inkling account" ||
		!strings.Contains(notice.Text, "/login") || !strings.Contains(notice.Text, "/forgot-password") {
		t.Errorf("the owner should be told, with a way in: %+v", notice)
	}
	if strings.Contains(notice.Text, "Another-pass-77") {
		t.Error("the email must not repeat the password that was typed")
	}

	// nothing about the account changed: the old password still works, the new one does not
	if r := a.newClient().post("/login", map[string]string{"email": email, "password": goodPass}); r.Status != http.StatusOK {
		t.Errorf("the owner can still log in: %d", r.Status)
	}
	if r := a.newClient().post("/login", map[string]string{"email": email, "password": "Another-pass-77"}); r.Status != http.StatusUnauthorized {
		t.Errorf("the stranger's password must not work: %d", r.Status)
	}
}

func TestTheOwnerIsNotFloodedWithNotices(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	a.mail.waitFor(t, 1)
	for i := 0; i < 5; i++ {
		a.newClient().post("/signup", map[string]string{"email": email, "password": goodPass})
	}
	a.mail.waitFor(t, 2)
	time.Sleep(300 * time.Millisecond)
	if n := len(a.mail.all()); n != 2 {
		t.Errorf("one notice an hour is enough, but %d emails were sent in all", n-1)
	}
}

func TestSignupsForTheSameAddressAtOnceMakeOneAccount(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a.newClient().post("/signup", map[string]string{"email": email, "password": goodPass})
			mu.Lock()
			statuses[r.Status]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[http.StatusOK] != 12 {
		t.Errorf("every reply is the same, whoever got there first: %v", statuses)
	}
	if n := count(t, a.db, "SELECT count(*) FROM users WHERE email = ?", email); n != 1 {
		t.Errorf("exactly one account, found %d", n)
	}
}

func TestBadSignupsAreStillRefusedWhetherOrNotTheAddressIsTaken(t *testing.T) {
	a := newApp(t)
	taken := uniqueEmail()
	a.signup(taken)
	for _, email := range []string{taken, uniqueEmail()} {
		if r := a.newClient().post("/signup", map[string]string{"email": email, "password": "short"}); r.Status != http.StatusBadRequest {
			t.Errorf("a short password for %s: %d", email, r.Status)
		}
	}
}

// ---- what counts as a good enough password -----------------------------------------------------------

func TestWeakPasswordsAreRefused(t *testing.T) {
	a := newApp(t)
	weak := []string{
		"password123", "Password123", "PASSWORD123", "12345678", "1234567890", "qwertyuiop", "Qwerty123", "letmein123",
		"aaaaaaaa", "abababab", "12341234", "87654321", "klmnopqr", "asdfghjk", "zyxwvuts", "passw0rd", "Inkling123",
	}
	for _, password := range weak {
		r := a.newClient().post("/signup", map[string]string{"email": uniqueEmail(), "password": password})
		if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "too common") {
			t.Errorf("%q: %d %s", password, r.Status, r.Body)
		}
	}
	// a password built from the email name
	r := a.newClient().post("/signup", map[string]string{"email": "alexandra@example.com", "password": "Alexandra-2024!"})
	if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "email name") {
		t.Errorf("a password with the email name in it: %d %s", r.Status, r.Body)
	}
}

func TestOrdinaryGoodPasswordsAreAccepted(t *testing.T) {
	a := newApp(t)
	for _, password := range []string{"correct-horse-battery", "T7#vq9!zL2", "Tr0ub4dor&3x", "a long sentence with spaces", "Sarajevo-Zagreb-7", "joNas1234x"} {
		r := a.newClient().post("/signup", map[string]string{"email": uniqueEmail(), "password": password})
		if r.Status != http.StatusOK {
			t.Errorf("%q: %d %s", password, r.Status, r.Body)
		}
	}
	// a short name inside the password is fine: "jo" is in too many ordinary words
	if r := a.newClient().post("/signup", map[string]string{"email": "jo@example.com", "password": "Journey-to-9-lakes"}); r.Status != http.StatusOK {
		t.Errorf("a two-letter name: %d %s", r.Status, r.Body)
	}
}

func TestChangingAndResettingAPasswordHoldsTheSameLine(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	r := cl.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": "password123"})
	if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "too common") {
		t.Errorf("changing to a common password: %d %s", r.Status, r.Body)
	}
}

// ---- how the passwords are kept --------------------------------------------------------------------------

func TestPasswordsAreHashedAtTheConfiguredCost(t *testing.T) {
	withPasswordCost(t, 6)
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	if cost, err := bcrypt.Cost([]byte(hashOf(t, a, email))); err != nil || cost != 6 {
		t.Errorf("cost = %d (%v), want 6", cost, err)
	}
	if auth.NeedsRehash(hashOf(t, a, email)) {
		t.Error("a hash made at the current cost needs no upgrade")
	}
}

func TestAnOldPasswordHashIsUpgradedWhenItsOwnerLogsIn(t *testing.T) {
	withPasswordCost(t, 4)
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email) // made at cost 4: an "old" hash
	old := hashOf(t, a, email)
	if cost, _ := bcrypt.Cost([]byte(old)); cost != 4 {
		t.Fatalf("setup: cost %d", cost)
	}

	auth.PasswordCost = 6 // the work factor was raised
	if r := a.newClient().post("/login", map[string]string{"email": email, "password": "wrong-password-1"}); r.Status != http.StatusUnauthorized {
		t.Fatalf("a wrong password: %d", r.Status)
	}
	if hashOf(t, a, email) != old {
		t.Error("a wrong password must not change anything")
	}

	cl := a.newClient()
	a.login(cl, email, goodPass)
	upgraded := hashOf(t, a, email)
	if cost, _ := bcrypt.Cost([]byte(upgraded)); cost != 6 {
		t.Errorf("after logging in the hash should be at cost 6, it is %d", cost)
	}
	// and it is a hash of the same password
	if bcrypt.CompareHashAndPassword([]byte(upgraded), []byte(goodPass)) != nil {
		t.Error("the upgraded hash must still match the password")
	}
	a.login(a.newClient(), email, goodPass)
	if hashOf(t, a, email) != upgraded {
		t.Error("a hash that is already up to date is left alone")
	}
}
