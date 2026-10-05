package middleware

import (
	"ai-summarizer/go-api/models"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

// RequestIDHeader carries the id that ties together every log line for one request, across
// services: the Go API passes it to the AI service, which logs it too.
const RequestIDHeader = "X-Request-ID"

const requestIDKey = "request_id"

// Accept ids from callers only if they are short and boring, so they are safe to log and echo.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown-request"
	}
	return hex.EncodeToString(b)
}

// RequestID gives every request an id (reusing a well-formed incoming one), exposes it to
// handlers via RequestIDFrom, and returns it in the response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// RequestIDFrom returns the current request's id, or "" outside a request.
func RequestIDFrom(c *gin.Context) string {
	if id, ok := c.Get(requestIDKey); ok {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}

// AccessLog writes one structured line per request. It logs the route pattern rather than the
// raw URL, and never the query string or headers, so tokens and user text cannot end up in logs.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()

		attrs := []any{
			"request_id", RequestIDFrom(c),
			"method", c.Request.Method,
			"route", route,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
			"client_ip", c.ClientIP(),
		}
		if user, ok := c.Get("user"); ok {
			if u, ok := user.(models.User); ok {
				attrs = append(attrs, "user_id", u.ID)
			}
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("request", attrs...)
		case status >= http.StatusBadRequest:
			logger.Warn("request", attrs...)
		default:
			logger.Info("request", attrs...)
		}
	}
}

// Recover turns a panic in a handler into a logged 500 instead of a dropped connection.
func Recover(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		logger.Error("panic", "request_id", RequestIDFrom(c), "panic", err)
		// A no-op unless Sentry was set up. Only the panic and the request id are reported.
		sentry.CurrentHub().WithScope(func(scope *sentry.Scope) {
			scope.SetTag("request_id", RequestIDFrom(c))
			sentry.CaptureException(fmt.Errorf("panic: %v", err))
		})
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	})
}
