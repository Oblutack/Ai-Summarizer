package controllers

import (
	"ai-summarizer/go-api/initializers"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const readyCheckTimeout = 2 * time.Second

// Check is one dependency probed by the readiness endpoint.
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Healthz is the liveness probe: the process is up and serving HTTP. It deliberately does not
// touch dependencies, so a database or AI outage doesn't get the container restarted.
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz is the readiness probe: every dependency answered within the timeout.
func Readyz(checks ...Check) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyCheckTimeout)
		defer cancel()

		results := make(gin.H, len(checks))
		ready := true
		for _, check := range checks {
			if err := check.Run(ctx); err != nil {
				ready = false
				results[check.Name] = "unavailable"
				_ = c.Error(fmt.Errorf("readiness: %s: %w", check.Name, err))
			} else {
				results[check.Name] = "ok"
			}
		}

		status := http.StatusOK
		state := "ready"
		if !ready {
			status = http.StatusServiceUnavailable
			state = "not ready"
		}
		c.JSON(status, gin.H{"status": state, "checks": results})
	}
}

// DatabaseCheck pings Postgres.
func DatabaseCheck() Check {
	return Check{Name: "database", Run: func(ctx context.Context) error {
		sqlDB, err := initializers.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}}
}

// AIServiceCheck calls the AI service's liveness endpoint.
func AIServiceCheck() Check {
	return Check{Name: "ai_service", Run: func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, aiServiceURL("/healthz"), nil)
		if err != nil {
			return err
		}
		resp, err := aiHTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New(resp.Status)
		}
		return nil
	}}
}
