package mailer

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

const appName = "Inkling"

// link builds an absolute URL to a frontend page carrying the token. The base URL comes from
// FRONTEND_URL; the token goes in the query string of our own page, which sends it to the API.
func link(frontendURL, path, token string) string {
	return strings.TrimRight(frontendURL, "/") + path + "?token=" + url.QueryEscape(token)
}

func wrap(heading, intro, button, href, outro string) (text, htmlBody string) {
	text = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s", heading, intro, href, outro, appName)
	htmlBody = fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:520px;margin:auto;color:#222">
<h2>%s</h2>
<p>%s</p>
<p><a href="%s" style="display:inline-block;background:#222;color:#f5f0e6;padding:12px 22px;text-decoration:none;border-radius:6px">%s</a></p>
<p style="font-size:13px;color:#555">If the button doesn't work, copy this link into your browser:<br>%s</p>
<p style="font-size:13px;color:#555">%s</p>
</div>`, html.EscapeString(heading), html.EscapeString(intro), html.EscapeString(href), html.EscapeString(button), html.EscapeString(href), html.EscapeString(outro))
	return text, htmlBody
}

// VerifyEmail is sent after signup.
func VerifyEmail(to, frontendURL, token string) Message {
	href := link(frontendURL, "/verify-email", token)
	text, body := wrap("Confirm your email", "Thanks for signing up. Please confirm this email address to finish setting up your account.",
		"Confirm email", href, "This link expires in 24 hours. If you didn't create an account, you can ignore this email.")
	return Message{To: to, Subject: "Confirm your email address", Text: text, HTML: body}
}

// ResetPassword is sent when someone asks to reset a password.
func ResetPassword(to, frontendURL, token string) Message {
	href := link(frontendURL, "/reset-password", token)
	text, body := wrap("Reset your password", "We received a request to reset the password for your account. Use the link below to choose a new one.",
		"Choose a new password", href, "This link expires in 1 hour and can be used once. If you didn't ask for this, ignore this email; your password hasn't changed.")
	return Message{To: to, Subject: "Reset your password", Text: text, HTML: body}
}

// PasswordChanged tells the account owner their password changed (so a hijacker can't do it silently).
func PasswordChanged(to string) Message {
	text := "Your password was just changed and all your other devices were signed out.\n\nIf this wasn't you, reset your password immediately and contact support.\n\n" + appName
	body := `<div style="font-family:Arial,sans-serif;max-width:520px;margin:auto;color:#222"><h2>Your password was changed</h2><p>All your other devices were signed out.</p><p style="font-size:13px;color:#555">If this wasn't you, reset your password immediately.</p></div>`
	return Message{To: to, Subject: "Your password was changed", Text: text, HTML: body}
}

// SummaryEmail sends a saved summary to its owner. The summary is Markdown; it is sent as it is, in a
// block that keeps its line breaks, rather than being rendered, so nothing in it can become markup.
func SummaryEmail(to, title, summary string) Message {
	title = strings.Join(strings.Fields(title), " ")
	text := fmt.Sprintf("%s\n\n%s\n\n%s", title, summary, appName)
	body := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto;color:#222">
<h2>%s</h2>
<div style="white-space:pre-wrap;line-height:1.5">%s</div>
<p style="font-size:13px;color:#555">Sent from %s because you asked for it.</p>
</div>`, html.EscapeString(title), html.EscapeString(summary), appName)
	return Message{To: to, Subject: "Summary: " + title, Text: text, HTML: body}
}
