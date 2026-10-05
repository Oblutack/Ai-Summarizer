package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// TurnstileHeader carries the token produced by the Cloudflare Turnstile widget.
const TurnstileHeader = "X-Turnstile-Token"

// turnstileVerifyURL is a variable so tests can point it at a fake server.
var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var turnstileClient = &http.Client{Timeout: 5 * time.Second}

// Turnstile rejects requests that don't carry a valid Cloudflare Turnstile token, keeping bots off
// the unauthenticated endpoints (anonymous summaries, signup, password reset). It does nothing when
// TURNSTILE_SECRET is unset, so local development and deployments without it keep working.
//
// If Cloudflare cannot be reached the request is refused (fail closed): an outage should not become
// a way around the check.
func Turnstile() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := os.Getenv("TURNSTILE_SECRET")
		if secret == "" {
			c.Next()
			return
		}

		token := strings.TrimSpace(c.GetHeader(TurnstileHeader))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Please complete the human check and try again.", "code": "turnstile_required"})
			return
		}

		ok, err := verifyTurnstile(c.Request.Context(), secret, token, c.ClientIP())
		if err != nil {
			slog.Error("turnstile verification failed", "request_id", RequestIDFrom(c), "error", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Could not run the human check right now. Please try again in a moment."})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "The human check failed. Please try again.", "code": "turnstile_failed"})
			return
		}
		c.Next()
	}
}

func verifyTurnstile(ctx context.Context, secret, token, ip string) (bool, error) {
	form := url.Values{"secret": {secret}, "response": {token}}
	if ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := turnstileClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, &url.Error{Op: "siteverify", URL: turnstileVerifyURL, Err: http.ErrNotSupported}
	}

	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Success, nil
}
