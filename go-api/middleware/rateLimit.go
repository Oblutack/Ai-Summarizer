package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-key token bucket limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens added per second
	burst   float64
	now     func() time.Time
}

// NewRateLimiter allows `burst` requests at once, refilling `perMinute` tokens each minute.
func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow reports whether the key may make a request right now.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.evictStale(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}

	b.tokens += now.Sub(b.lastSeen).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evictStale drops buckets idle long enough to have fully refilled, bounding memory use.
func (l *RateLimiter) evictStale(now time.Time) {
	if len(l.buckets) < 1024 {
		return
	}
	idle := time.Duration(l.burst/l.rate) * time.Second
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > idle {
			delete(l.buckets, k)
		}
	}
}

// RateLimit rejects requests from a client IP that exceeds the limiter's budget.
func RateLimit(l *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please wait a moment and try again.",
			})
			return
		}
		c.Next()
	}
}

// RateLimitUser limits per signed-in user instead of per IP, so people sharing a network (an
// office, a university, a mobile carrier) don't throttle each other. Must run after RequireAuth.
func RateLimitUser(l *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "user:" + strconv.FormatUint(uint64(CurrentUser(c).ID), 10)
		if !l.Allow(key) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please wait a moment and try again.",
			})
			return
		}
		c.Next()
	}
}
