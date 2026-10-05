// Package server builds the HTTP router: every route and the middleware in front of it. It is a
// package of its own so the tests exercise exactly what production runs.
package server

import (
	"ai-summarizer/go-api/controllers"
	"ai-summarizer/go-api/metrics"
	"ai-summarizer/go-api/middleware"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Rates is how many requests each kind of caller may make: a burst, then a steady rate per minute.
type Rates struct {
	AuthPerMinute, AuthBurst                   int // account endpoints, per IP
	SummarizeIPPerMinute, SummarizeIPBurst     int // anonymous summaries, per IP
	SummarizeUserPerMinute, SummarizeUserBurst int // signed-in summaries, per user
	ChatPerMinute, ChatBurst                   int // chat, per user
	ExportPerMinute, ExportBurst               int // data export, per user
}

// DefaultRates are the production limits.
var DefaultRates = Rates{
	AuthPerMinute: 20, AuthBurst: 10,
	SummarizeIPPerMinute: 6, SummarizeIPBurst: 3,
	SummarizeUserPerMinute: 10, SummarizeUserBurst: 5,
	ChatPerMinute: 20, ChatBurst: 10,
	ExportPerMinute: 2, ExportBurst: 2,
}

// Scaled returns the limits multiplied by factor (RATE_LIMIT_MULTIPLIER), for deployments that need
// more headroom than the defaults, such as many users behind one corporate address. A factor
// below 1 leaves the limits unchanged.
func (r Rates) Scaled(factor int) Rates {
	if factor <= 1 {
		return r
	}
	return Rates{
		AuthPerMinute: r.AuthPerMinute * factor, AuthBurst: r.AuthBurst * factor,
		SummarizeIPPerMinute: r.SummarizeIPPerMinute * factor, SummarizeIPBurst: r.SummarizeIPBurst * factor,
		SummarizeUserPerMinute: r.SummarizeUserPerMinute * factor, SummarizeUserBurst: r.SummarizeUserBurst * factor,
		ChatPerMinute: r.ChatPerMinute * factor, ChatBurst: r.ChatBurst * factor,
		ExportPerMinute: r.ExportPerMinute * factor, ExportBurst: r.ExportBurst * factor,
	}
}

// NewRouter returns the configured router. Rate limiters are created fresh for each router, so
// every router (and every test) starts with full allowances.
func NewRouter(logger *slog.Logger, rates Rates) (*gin.Engine, error) {
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.AccessLog(logger), metrics.HTTP(), middleware.Recover(logger), middleware.SecurityHeaders())

	// Client IPs drive the rate limits. Behind a proxy such as Render, list its addresses in
	// TRUSTED_PROXIES (comma separated CIDRs) so X-Forwarded-For is only believed from it.
	if proxies := os.Getenv("TRUSTED_PROXIES"); proxies != "" {
		if err := r.SetTrustedProxies(strings.Split(proxies, ",")); err != nil {
			return nil, err
		}
	} else {
		logger.Warn("TRUSTED_PROXIES is not set: X-Forwarded-For is trusted from anyone, so client IPs (and IP rate limits) can be spoofed")
	}

	origins := middleware.AllowedOrigins()
	config := cors.DefaultConfig()
	config.AllowOrigins = origins
	config.AllowCredentials = true // the session cookie; only sent to the explicit origins above
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", middleware.TurnstileHeader}
	config.ExposeHeaders = []string{controllers.NextCursorHeader, middleware.RequestIDHeader, "X-Quota-Limit", "X-Quota-Remaining", "Retry-After"}
	r.Use(cors.New(config))
	r.Use(middleware.OriginCheck(origins)) // CSRF: cookie auth must only act for our own frontends

	// Per-IP limits protect the unauthenticated endpoints; signed-in users are limited per user, and
	// by their daily quota (summaries and chat are the expensive LLM calls).
	summarizeIPLimit := middleware.RateLimit(middleware.NewRateLimiter(rates.SummarizeIPPerMinute, rates.SummarizeIPBurst))
	authLimit := middleware.RateLimit(middleware.NewRateLimiter(rates.AuthPerMinute, rates.AuthBurst))
	summarizeUserLimit := middleware.RateLimitUser(middleware.NewRateLimiter(rates.SummarizeUserPerMinute, rates.SummarizeUserBurst))
	chatUserLimit := middleware.RateLimitUser(middleware.NewRateLimiter(rates.ChatPerMinute, rates.ChatBurst))
	exportLimit := middleware.RateLimitUser(middleware.NewRateLimiter(rates.ExportPerMinute, rates.ExportBurst))
	turnstile := middleware.Turnstile()

	fileBody := middleware.MaxBody(controllers.MaxPDFBytes + 1<<20)
	multiBody := middleware.MaxBody(controllers.MaxMultiBytes + 1<<20)
	textBody := middleware.MaxBody(controllers.MaxTextBytes)
	chatBody := middleware.MaxBody(controllers.MaxTextBytes)
	smallBody := middleware.MaxBody(16 << 10) // auth and account requests are tiny JSON

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello from Go API Gateway"})
	})
	r.GET("/healthz", controllers.Healthz)
	r.GET("/readyz", controllers.Readyz(controllers.DatabaseCheck(), controllers.AIServiceCheck()))
	r.GET("/options", controllers.Options)

	// Prometheus metrics, only when a token is configured, and only for callers presenting it.
	if token := os.Getenv("METRICS_TOKEN"); token != "" {
		r.GET("/metrics", metrics.Handler(token))
	}

	// Public: account access.
	r.POST("/signup", authLimit, smallBody, turnstile, controllers.Signup)
	r.POST("/login", authLimit, smallBody, controllers.Login)
	r.POST("/auth/google", authLimit, smallBody, controllers.GoogleLogin)
	r.POST("/auth/logout", authLimit, controllers.Logout)
	r.POST("/auth/forgot-password", authLimit, smallBody, turnstile, controllers.ForgotPassword)
	r.POST("/auth/reset-password", authLimit, smallBody, controllers.ResetPassword)
	r.POST("/auth/verify-email", authLimit, smallBody, controllers.VerifyEmail)

	// Public: anonymous summaries (nothing is saved).
	r.POST("/public/summarize", summarizeIPLimit, fileBody, turnstile, controllers.PublicSummarize)
	r.POST("/public/summarize-multiple", summarizeIPLimit, multiBody, turnstile, controllers.PublicSummarizeMultiple)
	r.POST("/public/summarize-text", summarizeIPLimit, textBody, turnstile, controllers.PublicSummarizeText)

	authorized := r.Group("/")
	authorized.Use(middleware.RequireAuth)
	{
		// The account.
		authorized.GET("/auth/me", controllers.Me)
		authorized.POST("/auth/logout-all", controllers.LogoutAll)
		authorized.GET("/auth/sessions", controllers.ListSessions)
		authorized.DELETE("/auth/sessions/:id", controllers.RevokeSession)
		authorized.POST("/auth/change-password", authLimit, smallBody, controllers.ChangePassword)
		authorized.POST("/auth/resend-verification", authLimit, controllers.ResendVerification)
		authorized.GET("/usage", controllers.Usage)
		authorized.GET("/account/export", exportLimit, controllers.ExportAccount)
		authorized.DELETE("/account", authLimit, smallBody, controllers.DeleteAccount)

		// Saved documents.
		authorized.GET("/documents", controllers.ListDocuments)
		authorized.DELETE("/documents/:id", controllers.DeleteDocument)

		// Work that costs an LLM call: signed in (and verified, when required), rate limited, then
		// charged against the daily quota last so rejected requests don't use up allowance.
		verified := authorized.Group("/", middleware.RequireVerifiedEmail)
		verified.POST("/summarize", summarizeUserLimit, fileBody, middleware.Quota(middleware.QuotaSummaries), controllers.CreateSummary)
		verified.POST("/summarize-multiple", summarizeUserLimit, multiBody, middleware.Quota(middleware.QuotaSummaries), controllers.CreateSummaryMultiple)
		verified.POST("/summarize-text", summarizeUserLimit, textBody, middleware.Quota(middleware.QuotaSummaries), controllers.CreateSummaryText)
		verified.POST("/documents/:id/chat", chatUserLimit, chatBody, middleware.Quota(middleware.QuotaChats), controllers.ChatWithDocument)
	}

	return r, nil
}
