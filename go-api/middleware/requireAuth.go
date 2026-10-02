package middleware

import (
	"ai-summarizer/go-api/auth"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context keys set by RequireAuth.
const (
	UserKey      = "user"
	SessionIDKey = "session_id"
)

// RequireAuth accepts a valid session from the session cookie (browsers) or an
// "Authorization: Bearer <session token>" header (scripts and API clients). Unknown, expired and
// revoked sessions are indistinguishable to the caller.
func RequireAuth(c *gin.Context) {
	session, user, err := auth.LookupSession(auth.TokenFromRequest(c))
	if err != nil {
		if err != auth.ErrInvalidSession {
			slog.Error("session lookup failed", "request_id", RequestIDFrom(c), "error", err)
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Please log in again.", "code": "unauthenticated"})
		return
	}

	c.Set(UserKey, *user)
	c.Set(SessionIDKey, session.ID)
	c.Next()
}

// RequireVerifiedEmail blocks users who haven't confirmed their email. It only does anything when
// REQUIRE_EMAIL_VERIFICATION=true, so deployments without a mail provider keep working.
func RequireVerifiedEmail(c *gin.Context) {
	if strings.ToLower(os.Getenv("REQUIRE_EMAIL_VERIFICATION")) != "true" {
		c.Next()
		return
	}
	user := CurrentUser(c)
	if !user.EmailVerified() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "Please confirm your email address to use this feature. We sent you a link when you signed up.",
			"code":  "email_not_verified",
		})
		return
	}
	c.Next()
}
