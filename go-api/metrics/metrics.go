// Package metrics exposes Prometheus metrics: how many requests the API serves and how fast, how
// the AI service behaves from the gateway's point of view, and the account and quota events that
// matter operationally.
//
// Labels are kept to a small fixed set (route patterns, never raw URLs or user input) so the number
// of time series stays bounded no matter what clients send.
package metrics

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is separate from Prometheus' global default so tests can create many routers without
// colliding, and so only metrics we chose to publish are exposed.
var Registry = prometheus.NewRegistry()

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests served, by method, route pattern and status code.",
	}, []string{"method", "route", "status"})

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time to serve an HTTP request, by method and route pattern. Streaming responses count until the stream ends.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"method", "route"})

	httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "HTTP requests currently being served.",
	})

	aiRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_service_requests_total",
		Help: "Calls from the gateway to the AI service, by endpoint and outcome (ok, unavailable, upstream_error).",
	}, []string{"endpoint", "outcome"})

	aiDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_service_response_seconds",
		Help:    "Time until the AI service answered (for streams: until the first response, not the end of the stream).",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 20, 40, 80},
	}, []string{"endpoint"})

	accountEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "account_events_total",
		Help: "Sign-ups, logins and other account events, by event name.",
	}, []string{"event"})

	quotaRejections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "quota_rejections_total",
		Help: "Requests refused because the user reached a daily quota, by quota kind.",
	}, []string{"kind"})
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequests, httpDuration, httpInFlight, aiRequests, aiDuration, accountEvents, quotaRejections,
	)
}

// HTTP records a count and a duration for every request, labelled by route pattern.
func HTTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		httpInFlight.Inc()
		defer httpInFlight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched" // 404s: don't let scanners create a series per probed URL
		}
		httpRequests.WithLabelValues(c.Request.Method, route, statusLabel(c.Writer.Status())).Inc()
		httpDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

// Whole codes (not classes) are worth keeping: 401, 403 and 429 tell different stories.
func statusLabel(code int) string { return strconv.Itoa(code) }

// AIRequest records one call to the AI service. endpoint is the path called (a small fixed set).
func AIRequest(endpoint, outcome string, took time.Duration) {
	aiRequests.WithLabelValues(endpoint, outcome).Inc()
	aiDuration.WithLabelValues(endpoint).Observe(took.Seconds())
}

// AccountEvent counts an account event such as "login_success" or "signup".
func AccountEvent(event string) { accountEvents.WithLabelValues(event).Inc() }

// QuotaRejected counts a request refused because a daily quota was used up.
func QuotaRejected(kind string) { quotaRejections.WithLabelValues(kind).Inc() }

// Handler serves the metrics, but only to a caller presenting the bearer token. The endpoint is
// meant for a Prometheus scraper, not browsers, and an unauthenticated one would leak traffic
// patterns. An empty token disables the endpoint entirely (see Enabled).
func Handler(token string) gin.HandlerFunc {
	serve := promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		given, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		serve.ServeHTTP(c.Writer, c.Request)
	}
}
