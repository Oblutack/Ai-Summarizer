package auth

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CookieName is the session cookie.
const CookieName = "session"

// CookieConfig controls how the session cookie is issued.
//
//	COOKIE_SECURE:   true (default) | false. Must be true in production; set false only for
//	                 plain-http local development on browsers that refuse Secure cookies there.
//	COOKIE_SAMESITE: lax (default) | strict | none. Use "none" only when the frontend and the API
//	                 are on different sites (e.g. vercel.app and onrender.com); it requires Secure.
//	COOKIE_DOMAIN:   optional, e.g. ".example.com" when the app and API share a parent domain.
type CookieConfig struct {
	Secure   bool
	SameSite http.SameSite
	Domain   string
}

// LoadCookieConfig reads the cookie settings from the environment.
func LoadCookieConfig() CookieConfig {
	cfg := CookieConfig{
		Secure: strings.ToLower(os.Getenv("COOKIE_SECURE")) != "false",
		Domain: os.Getenv("COOKIE_DOMAIN"),
	}
	switch strings.ToLower(os.Getenv("COOKIE_SAMESITE")) {
	case "strict":
		cfg.SameSite = http.SameSiteStrictMode
	case "none":
		cfg.SameSite = http.SameSiteNoneMode
		cfg.Secure = true // browsers reject SameSite=None cookies that are not Secure
	default:
		cfg.SameSite = http.SameSiteLaxMode
	}
	return cfg
}

// Cookie is the active configuration; tests may replace it.
var Cookie = LoadCookieConfig()

// SetSessionCookie sends the session token. HttpOnly keeps it out of reach of page scripts, so
// an XSS bug cannot steal the login.
func SetSessionCookie(c *gin.Context, token string, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Domain:   Cookie.Domain,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   Cookie.Secure,
		SameSite: Cookie.SameSite,
	})
}

// ClearSessionCookie tells the browser to forget the session.
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Domain:   Cookie.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   Cookie.Secure,
		SameSite: Cookie.SameSite,
	})
}

// TokenFromRequest returns the session token from the cookie, or from an
// "Authorization: Bearer" header for non-browser clients.
func TokenFromRequest(c *gin.Context) string {
	if cookie, err := c.Request.Cookie(CookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " "); ok && scheme == "Bearer" {
		return token
	}
	return ""
}
