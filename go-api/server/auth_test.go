package server

import (
	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/models"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ---- signup and email verification ---------------------------------------------------------

func TestSignupValidation(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	cases := map[string]map[string]string{
		"bad email":       {"email": "not-an-email", "password": goodPass},
		"short password":  {"email": uniqueEmail(), "password": "short"},
		"common password": {"email": uniqueEmail(), "password": "password123"},
		"empty":           {"email": "", "password": ""},
	}
	for name, body := range cases {
		if r := cl.post("/signup", body); r.Status != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400 (%s)", name, r.Status, r.Body)
		}
	}
	if len(a.mail.all()) != 0 {
		t.Error("rejected signups must not send email")
	}
}

func TestSignupRejectsDuplicatesCaseInsensitively(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	if r := a.newClient().post("/signup", map[string]string{"email": strings.ToUpper(email), "password": goodPass}); r.Status != http.StatusConflict {
		t.Errorf("got %d, want 409", r.Status)
	}
}

func TestSignupDoesNotLogYouIn(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	r := cl.post("/signup", map[string]string{"email": uniqueEmail(), "password": goodPass})
	if r.Status != 200 || r.cookie("session") != nil {
		t.Fatalf("signup should create the account only: %d cookie=%v", r.Status, r.cookie("session"))
	}
	if cl.get("/auth/me").Status != http.StatusUnauthorized {
		t.Error("no session should exist after signup")
	}
}

func TestSignupSendsAVerificationLinkThatConfirmsTheEmail(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)

	mail := a.mail.waitFor(t, 1)[0]
	if mail.To != email || !strings.Contains(mail.Text, appOrigin+"/verify-email?token=") {
		t.Fatalf("unexpected verification email: %+v", mail)
	}
	if a.userByEmail(email).EmailVerified() {
		t.Fatal("a new account must start unverified")
	}

	token := tokenIn(t, mail)
	cl := a.newClient()
	if r := cl.post("/auth/verify-email", map[string]string{"token": token}); r.Status != 200 {
		t.Fatalf("verify: %d %s", r.Status, r.Body)
	}
	if !a.userByEmail(email).EmailVerified() {
		t.Error("the account should now be verified")
	}

	// the link is single-use
	if r := cl.post("/auth/verify-email", map[string]string{"token": token}); r.Status != http.StatusBadRequest {
		t.Errorf("reusing a verification link: got %d, want 400", r.Status)
	}
}

func TestInvalidAndWrongPurposeVerificationTokensAreRejected(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	for _, tok := range []string{"", "nonsense", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if r := cl.post("/auth/verify-email", map[string]string{"token": tok}); r.Status != http.StatusBadRequest {
			t.Errorf("token %q: got %d, want 400", tok, r.Status)
		}
	}

	// a password-reset token must not verify an email
	email := uniqueEmail()
	a.signup(email)
	cl.post("/auth/forgot-password", map[string]string{"email": email})
	reset := a.mail.waitFor(t, 2)[1]
	if r := cl.post("/auth/verify-email", map[string]string{"token": tokenIn(t, reset)}); r.Status != http.StatusBadRequest {
		t.Errorf("a reset token must not work as a verification token: %d", r.Status)
	}
}

func TestExpiredVerificationTokenIsRejected(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	token := tokenIn(t, a.mail.waitFor(t, 1)[0])

	a.db.Exec("UPDATE email_tokens SET expires_at = ? WHERE purpose = 'verify_email'", time.Now().Add(-time.Minute))
	if r := a.newClient().post("/auth/verify-email", map[string]string{"token": token}); r.Status != http.StatusBadRequest {
		t.Errorf("got %d, want 400", r.Status)
	}
}

func TestResendVerificationIsThrottledAndStopsOnceVerified(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	a.mail.waitFor(t, 1) // the signup email

	// signup just issued a token, so an immediate resend is refused
	if r := cl.post("/auth/resend-verification", nil); r.Status != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", r.Status)
	}

	a.db.Exec("UPDATE email_tokens SET created_at = ?", time.Now().Add(-2*time.Minute))
	if r := cl.post("/auth/resend-verification", nil); r.Status != 200 {
		t.Fatalf("resend after the cooldown: %d %s", r.Status, r.Body)
	}
	mails := a.mail.waitFor(t, 2)

	// the new link replaced the old one
	if r := cl.post("/auth/verify-email", map[string]string{"token": tokenIn(t, mails[0])}); r.Status != http.StatusBadRequest {
		t.Error("an older verification link should stop working once a newer one is issued")
	}
	if r := cl.post("/auth/verify-email", map[string]string{"token": tokenIn(t, mails[1])}); r.Status != 200 {
		t.Fatal("the newest link should work")
	}

	if r := cl.post("/auth/resend-verification", nil); r.Status != 200 || !strings.Contains(r.Str("message"), "already") {
		t.Errorf("verified users should be told so: %d %s", r.Status, r.Body)
	}
	if len(a.mail.all()) != 2 {
		t.Errorf("no further email should be sent, got %d total", len(a.mail.all()))
	}
	_ = email
}

func TestUnverifiedUsersAreBlockedFromSummariesOnlyWhenRequired(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	if r := cl.post("/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
		t.Fatalf("verification not required: got %d %s", r.Status, r.Body)
	}

	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "true")
	r := cl.post("/summarize-text", map[string]string{"text": "hello world"})
	if r.Status != http.StatusForbidden || r.Str("code") != "email_not_verified" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	// but they can still reach their account and ask for another email
	if cl.get("/auth/me").Status != 200 || cl.get("/documents").Status != 200 {
		t.Error("account pages must stay reachable while unverified")
	}

	token := tokenIn(t, a.mail.waitFor(t, 1)[0])
	cl.post("/auth/verify-email", map[string]string{"token": token})
	if r := cl.post("/summarize-text", map[string]string{"text": "hello world"}); r.Status != 200 {
		t.Errorf("after verifying: got %d %s", r.Status, r.Body)
	}
}

// ---- login, sessions, logout ---------------------------------------------------------------

func TestLoginSetsAnHttpOnlyCookieAndNeverReturnsTheToken(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)

	r := a.newClient().post("/login", map[string]string{"email": email, "password": goodPass})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	cookie := r.cookie("session")
	if cookie == nil || !cookie.HttpOnly || cookie.Path != "/" || cookie.MaxAge < 24*3600 {
		t.Fatalf("session cookie wrong: %+v", cookie)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v", cookie.SameSite)
	}
	if strings.Contains(string(r.Body), cookie.Value) {
		t.Fatal("the session token must never appear in the response body (page scripts could read it)")
	}
	user, _ := r.JSON()["user"].(map[string]any)
	if user["email"] != email || user["emailVerified"] != false {
		t.Errorf("user view = %v", user)
	}
	if _, leaked := user["password"]; leaked {
		t.Fatal("password hash leaked")
	}
}

func TestOnlyTheTokenHashIsStored(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	token := cl.sessionToken()
	if token == "" {
		t.Fatal("no session")
	}
	if count(t, a.db, "SELECT count(*) FROM sessions WHERE token_hash = ?", token) != 0 {
		t.Fatal("the raw token must not be stored")
	}
	if count(t, a.db, "SELECT count(*) FROM sessions") != 1 {
		t.Fatal("expected exactly one session row")
	}
}

func TestWrongPasswordAndUnknownUserAreIndistinguishable(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	cl := a.newClient()

	wrong := cl.post("/login", map[string]string{"email": email, "password": "Wrong-password-1"})
	unknown := cl.post("/login", map[string]string{"email": uniqueEmail(), "password": "Wrong-password-1"})
	if wrong.Status != 401 || unknown.Status != 401 || string(wrong.Body) != string(unknown.Body) {
		t.Fatalf("responses differ: %d %s vs %d %s", wrong.Status, wrong.Body, unknown.Status, unknown.Body)
	}
	if wrong.cookie("session") != nil {
		t.Error("a failed login must not set a cookie")
	}
}

func TestLoginIsCaseInsensitiveOnEmail(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	a.login(a.newClient(), strings.ToUpper(email), goodPass)
}

func TestRepeatedFailedLoginsForOneAccountAreThrottled(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	cl := a.newClient()

	var last reply
	got429 := false
	for i := 0; i < 15; i++ {
		last = cl.post("/login", map[string]string{"email": email, "password": "Wrong-password-1"})
		if last.Status == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 || last.Header.Get("Retry-After") == "" {
		t.Fatalf("guessing against one account should be throttled (last: %d)", last.Status)
	}
	// even the right password is refused while throttled
	if r := cl.post("/login", map[string]string{"email": email, "password": goodPass}); r.Status != http.StatusTooManyRequests {
		t.Errorf("throttled account accepted a login: %d", r.Status)
	}
	// other accounts are unaffected
	a.login(a.newClient(), func() string { e := uniqueEmail(); a.signup(e); return e }(), goodPass)
}

func TestMeRequiresASession(t *testing.T) {
	a := newApp(t)
	r := a.newClient().get("/auth/me")
	if r.Status != http.StatusUnauthorized || r.Str("code") != "unauthenticated" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}

	cl, email := a.newUser()
	r = cl.get("/auth/me")
	user, _ := r.JSON()["user"].(map[string]any)
	if r.Status != 200 || user["email"] != email {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestBearerTokensWorkForNonBrowserClients(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	token := cl.sessionToken()

	bare := a.newClient() // no cookies
	r := bare.req(http.MethodGet, "/auth/me", nil, "Authorization", "Bearer "+token)
	if user, _ := r.JSON()["user"].(map[string]any); r.Status != 200 || user["email"] != email {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
	if r := bare.req(http.MethodGet, "/auth/me", nil, "Authorization", "Bearer not-a-real-token"); r.Status != 401 {
		t.Errorf("garbage bearer token: got %d", r.Status)
	}
}

func TestLogoutRevokesTheSessionServerSide(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	token := cl.sessionToken()

	r := cl.post("/auth/logout", nil)
	if r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	if c := r.cookie("session"); c == nil || c.MaxAge >= 0 {
		t.Fatalf("logout should clear the cookie: %+v", c)
	}

	// The key property: a copy of the old token (stolen, or sitting in a cookie jar elsewhere) is dead.
	stolen := a.newClient()
	if r := stolen.req(http.MethodGet, "/auth/me", nil, "Authorization", "Bearer "+token); r.Status != http.StatusUnauthorized {
		t.Fatalf("a logged-out token still works: %d", r.Status)
	}
}

func TestLogoutWorksWithoutAValidSession(t *testing.T) {
	a := newApp(t)
	cl := a.newClient()
	if r := cl.post("/auth/logout", nil); r.Status != 200 {
		t.Errorf("logging out while logged out: %d", r.Status)
	}
	if r := cl.req(http.MethodPost, "/auth/logout", nil, "Authorization", "Bearer garbage"); r.Status != 200 {
		t.Errorf("logging out with a bad token: %d", r.Status)
	}
}

func TestLogoutAllEndsEveryDevice(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	laptop, phone := a.newClient(), a.newClient()
	a.login(laptop, email, goodPass)
	a.login(phone, email, goodPass)

	if r := laptop.post("/auth/logout-all", nil); r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	if laptop.get("/auth/me").Status != 401 || phone.get("/auth/me").Status != 401 {
		t.Error("every session should be revoked")
	}
	if count(t, a.db, "SELECT count(*) FROM sessions WHERE revoked_at IS NULL") != 0 {
		t.Error("revoked sessions are kept for audit but must be marked")
	}
}

func TestSessionListAndRevokeASingleDevice(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	laptop, phone := a.newClient(), a.newClient()
	a.login(laptop, email, goodPass)
	phone.req(http.MethodPost, "/login", map[string]string{"email": email, "password": goodPass}, "User-Agent", "Mozilla/5.0 (iPhone)")

	r := laptop.get("/auth/sessions")
	var list []map[string]any
	_ = jsonUnmarshal(r.Body, &list)
	if r.Status != 200 || len(list) != 2 {
		t.Fatalf("expected 2 sessions: %d %s", r.Status, r.Body)
	}
	var phoneID float64
	currents := 0
	for _, s := range list {
		if s["current"] == true {
			currents++
		}
		if strings.Contains(s["userAgent"].(string), "iPhone") {
			phoneID = s["id"].(float64)
		}
	}
	if currents != 1 || phoneID == 0 {
		t.Fatalf("expected exactly one current session and the phone: %v", list)
	}

	if r := laptop.delete("/auth/sessions/"+itoa(int(phoneID)), nil); r.Status != 200 {
		t.Fatalf("revoke: %d %s", r.Status, r.Body)
	}
	if phone.get("/auth/me").Status != 401 {
		t.Error("the revoked device should be signed out")
	}
	if laptop.get("/auth/me").Status != 200 {
		t.Error("the other device must stay signed in")
	}
	if r := laptop.delete("/auth/sessions/"+itoa(int(phoneID)), nil); r.Status != http.StatusNotFound {
		t.Errorf("revoking twice: got %d, want 404", r.Status)
	}
}

func TestCannotRevokeSomeoneElsesSession(t *testing.T) {
	a := newApp(t)
	alice, aliceEmail := a.newUser()
	bob, _ := a.newUser()

	var id uint
	a.db.Raw("SELECT id FROM sessions WHERE user_id = ?", a.userByEmail(aliceEmail).ID).Scan(&id)
	if r := bob.delete("/auth/sessions/"+itoa(int(id)), nil); r.Status != http.StatusNotFound {
		t.Errorf("got %d, want 404", r.Status)
	}
	if alice.get("/auth/me").Status != 200 {
		t.Error("Alice's session must be untouched")
	}
}

func TestExpiredSessionsAreRejected(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()
	a.db.Exec("UPDATE sessions SET expires_at = ?", time.Now().Add(-time.Minute))
	if cl.get("/auth/me").Status != http.StatusUnauthorized {
		t.Error("an expired session must not authenticate")
	}
}

func TestSessionsSlideButHaveAHardLimit(t *testing.T) {
	a := newApp(t)
	cl, _ := a.newUser()

	// used after being idle a while: expiry moves out to a full TTL from now
	a.db.Exec("UPDATE sessions SET last_used_at = ?, expires_at = ?", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if cl.get("/auth/me").Status != 200 {
		t.Fatal("session should still be valid")
	}
	var expires time.Time
	a.db.Raw("SELECT expires_at FROM sessions").Scan(&expires)
	if time.Until(expires) < 29*24*time.Hour {
		t.Errorf("a used session should be extended to ~30 days, expires in %v", time.Until(expires))
	}

	// but never beyond 90 days since it was created
	a.db.Exec("UPDATE sessions SET created_at = ?, last_used_at = ?", time.Now().Add(-89*24*time.Hour), time.Now().Add(-time.Hour))
	cl.get("/auth/me")
	a.db.Raw("SELECT expires_at FROM sessions").Scan(&expires)
	if time.Until(expires) > 24*time.Hour+time.Minute {
		t.Errorf("hard limit exceeded: expires in %v", time.Until(expires))
	}
}

// ---- passwords -----------------------------------------------------------------------------

func TestChangePasswordRevokesOtherDevicesButKeepsThisOne(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	laptop, phone := a.newClient(), a.newClient()
	a.login(laptop, email, goodPass)
	a.login(phone, email, goodPass)
	a.mail.waitFor(t, 1)

	const newPass = "Another-strong-pass-7"
	if r := laptop.post("/auth/change-password", map[string]string{"currentPassword": "Wrong-password-1", "newPassword": newPass}); r.Status != http.StatusForbidden || r.Str("code") != "wrong_password" {
		t.Errorf("wrong current password must be 403 (not 401, which means the session ended): got %d %s", r.Status, r.Body)
	}
	if laptop.get("/auth/me").Status != 200 {
		t.Error("a mistyped password must not end the session")
	}
	if r := laptop.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": "short"}); r.Status != http.StatusBadRequest {
		t.Errorf("weak new password: got %d", r.Status)
	}
	if r := laptop.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": goodPass}); r.Status != http.StatusBadRequest {
		t.Errorf("same password: got %d", r.Status)
	}

	if r := laptop.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": newPass}); r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	if phone.get("/auth/me").Status != 401 {
		t.Error("other devices must be signed out when the password changes")
	}
	if laptop.get("/auth/me").Status != 200 {
		t.Error("the device that changed it stays signed in")
	}
	if r := a.newClient().post("/login", map[string]string{"email": email, "password": goodPass}); r.Status != 401 {
		t.Error("the old password must stop working")
	}
	a.login(a.newClient(), email, newPass)

	notice := a.mail.waitFor(t, 2)[1]
	if notice.To != email || !strings.Contains(notice.Subject, "password was changed") {
		t.Errorf("expected a security notice, got %+v", notice)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	victim := a.newClient()
	a.login(victim, email, goodPass)
	a.mail.waitFor(t, 1)

	cl := a.newClient()
	r := cl.post("/auth/forgot-password", map[string]string{"email": strings.ToUpper(email)})
	if r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	mail := a.mail.waitFor(t, 2)[1]
	if mail.To != email || !strings.Contains(mail.Text, appOrigin+"/reset-password?token=") {
		t.Fatalf("unexpected reset email: %+v", mail)
	}
	token := tokenIn(t, mail)

	// a weak password is rejected without burning the link
	if r := cl.post("/auth/reset-password", map[string]string{"token": token, "password": "short"}); r.Status != 400 {
		t.Errorf("weak password: %d", r.Status)
	}
	const newPass = "Brand-new-pass-42"
	if r := cl.post("/auth/reset-password", map[string]string{"token": token, "password": newPass}); r.Status != 200 {
		t.Fatalf("reset: %d %s", r.Status, r.Body)
	}

	// the link is single-use
	if r := cl.post("/auth/reset-password", map[string]string{"token": token, "password": "Yet-another-pass-1"}); r.Status != 400 {
		t.Errorf("reusing a reset link: %d", r.Status)
	}
	// existing logins are cut off (the reason many people reset)
	if victim.get("/auth/me").Status != 401 {
		t.Error("a password reset must sign the account out everywhere")
	}
	if a.newClient().post("/login", map[string]string{"email": email, "password": goodPass}).Status != 401 {
		t.Error("old password must stop working")
	}
	a.login(a.newClient(), email, newPass)

	// a successful reset proves they own the inbox
	if !a.userByEmail(email).EmailVerified() {
		t.Error("resetting through an emailed link should verify the email")
	}
	// and they're told
	if got := a.mail.waitFor(t, 3)[2]; !strings.Contains(got.Subject, "password was changed") {
		t.Errorf("expected a notice, got %+v", got)
	}
}

func TestForgotPasswordRevealsNothingAboutWhoHasAnAccount(t *testing.T) {
	a := newApp(t)
	known := uniqueEmail()
	a.signup(known)
	a.mail.waitFor(t, 1)
	cl := a.newClient()

	yes := cl.post("/auth/forgot-password", map[string]string{"email": known})
	no := cl.post("/auth/forgot-password", map[string]string{"email": uniqueEmail()})
	empty := cl.post("/auth/forgot-password", map[string]string{"email": ""})
	if yes.Status != 200 || no.Status != 200 || empty.Status != 200 || string(yes.Body) != string(no.Body) || string(no.Body) != string(empty.Body) {
		t.Fatalf("responses differ:\n%d %s\n%d %s\n%d %s", yes.Status, yes.Body, no.Status, no.Body, empty.Status, empty.Body)
	}

	a.mail.waitFor(t, 2)
	time.Sleep(150 * time.Millisecond) // give any wrongly-sent email time to show up
	if mails := a.mail.all(); len(mails) != 2 || mails[1].To != known {
		t.Errorf("only the real account should get an email: %+v", mails)
	}
}

func TestResetRequestsForOneAddressAreCapped(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	a.mail.waitFor(t, 1)
	cl := a.newClient()
	for i := 0; i < 8; i++ {
		if r := cl.post("/auth/forgot-password", map[string]string{"email": email}); r.Status != 200 {
			t.Fatalf("every attempt gets the same answer: %d", r.Status)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if n := len(a.mail.all()) - 1; n > 4 {
		t.Errorf("%d reset emails sent for one address, expected the cap to hold at about 3", n)
	}
}

func TestExpiredResetTokenIsRejected(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	cl := a.newClient()
	cl.post("/auth/forgot-password", map[string]string{"email": email})
	token := tokenIn(t, a.mail.waitFor(t, 2)[1])

	a.db.Exec("UPDATE email_tokens SET expires_at = ? WHERE purpose = 'reset_password'", time.Now().Add(-time.Minute))
	if r := cl.post("/auth/reset-password", map[string]string{"token": token, "password": "Brand-new-pass-42"}); r.Status != 400 {
		t.Errorf("got %d, want 400", r.Status)
	}
}

func TestNewResetLinkInvalidatesTheOldOne(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	cl := a.newClient()
	cl.post("/auth/forgot-password", map[string]string{"email": email})
	first := tokenIn(t, a.mail.waitFor(t, 2)[1])
	cl.post("/auth/forgot-password", map[string]string{"email": email})
	second := tokenIn(t, a.mail.waitFor(t, 3)[2])

	if r := cl.post("/auth/reset-password", map[string]string{"token": first, "password": "Brand-new-pass-42"}); r.Status != 400 {
		t.Error("the older link must stop working once a newer one is issued")
	}
	if r := cl.post("/auth/reset-password", map[string]string{"token": second, "password": "Brand-new-pass-42"}); r.Status != 200 {
		t.Errorf("the newest link should work: %d", r.Status)
	}
}

func TestChangePasswordIsNotAvailableToGoogleOnlyAccounts(t *testing.T) {
	a := newApp(t)
	cl, email := a.newUser()
	a.db.Exec("UPDATE users SET has_password = false WHERE email = ?", email)
	r := cl.post("/auth/change-password", map[string]string{"currentPassword": goodPass, "newPassword": "Another-strong-pass-7"})
	if r.Status != http.StatusBadRequest || !strings.Contains(r.Str("error"), "Google") {
		t.Errorf("got %d %s", r.Status, r.Body)
	}
}

// ---- Google accounts (the sign-in call itself needs Google; the account logic does not) -------

func TestGoogleAccountsAreVerifiedAndHaveNoPassword(t *testing.T) {
	a := newApp(t)
	user, err := controllers.FindOrCreateGoogleUser("new.google.user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.HasPassword || !user.EmailVerified() {
		t.Errorf("a Google account has no password and a verified email: %+v", user)
	}
	if a.userByEmail("new.google.user@example.com").HasPassword {
		t.Error("persisted has_password should be false")
	}
}

func TestGoogleSignInVerifiesAnExistingUnverifiedAccount(t *testing.T) {
	a := newApp(t)
	email := uniqueEmail()
	a.signup(email)
	if a.userByEmail(email).EmailVerified() {
		t.Fatal("precondition")
	}
	user, err := controllers.FindOrCreateGoogleUser(strings.ToUpper(email))
	if err != nil {
		t.Fatal(err)
	}
	var found models.User
	a.db.First(&found, user.ID)
	if !found.EmailVerified() || !found.HasPassword {
		t.Errorf("the existing password account should be verified and keep its password: %+v", found)
	}
}
