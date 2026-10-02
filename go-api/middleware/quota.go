package middleware

import (
	"ai-summarizer/go-api/initializers"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// What a quota counts.
const (
	QuotaSummaries = "summaries"
	QuotaChats     = "chats"
)

const quotaKey = "quota_claim"

// Default daily limits per signed-in user. 0 means unlimited.
const (
	DefaultSummariesPerDay = 50
	DefaultChatsPerDay     = 200
)

// QuotaLimit reads the daily limit for a kind from QUOTA_SUMMARIES_PER_DAY / QUOTA_CHAT_PER_DAY.
func QuotaLimit(kind string) int {
	name, def := "QUOTA_SUMMARIES_PER_DAY", DefaultSummariesPerDay
	if kind == QuotaChats {
		name, def = "QUOTA_CHAT_PER_DAY", DefaultChatsPerDay
	}
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v >= 0 {
		return v
	}
	return def
}

type quotaClaim struct {
	kind   string
	userID uint
	day    string
}

// The two SQL statements are fixed strings (one per column) so no user input is ever interpolated.
//
// The conditional upsert is atomic: concurrent requests cannot both squeeze under the limit, because
// the row is locked and the WHERE is evaluated against the current count. No row comes back when
// the limit has been reached.
const (
	claimSummarySQL = `INSERT INTO daily_usage (user_id, day, summaries) VALUES (?, CURRENT_DATE, 1)
		ON CONFLICT (user_id, day) DO UPDATE SET summaries = daily_usage.summaries + 1
		WHERE daily_usage.summaries < ? RETURNING summaries, day::text`
	claimChatSQL = `INSERT INTO daily_usage (user_id, day, chats) VALUES (?, CURRENT_DATE, 1)
		ON CONFLICT (user_id, day) DO UPDATE SET chats = daily_usage.chats + 1
		WHERE daily_usage.chats < ? RETURNING chats, day::text`
	refundSummarySQL = `UPDATE daily_usage SET summaries = GREATEST(summaries - 1, 0) WHERE user_id = ? AND day = ?::date`
	refundChatSQL    = `UPDATE daily_usage SET chats = GREATEST(chats - 1, 0) WHERE user_id = ? AND day = ?::date`
)

// Quota counts one use of the given kind against the signed-in user's daily allowance and refuses
// the request once it is spent. Handlers call RefundQuota when the work fails through no fault of
// the user, so an outage doesn't eat their allowance. Must run after RequireAuth.
func Quota(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := QuotaLimit(kind)
		if limit == 0 {
			c.Next()
			return
		}
		user := CurrentUser(c)

		query := claimSummarySQL
		if kind == QuotaChats {
			query = claimChatSQL
		}
		var used int
		var day string
		row := initializers.DB.Raw(query, user.ID, limit).Row()
		if err := row.Scan(&used, &day); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				slog.Error("quota check failed", "request_id", RequestIDFrom(c), "error", err)
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Could not check your usage right now. Please try again."})
				return
			}
			// No row came back: the limit has been reached.
			c.Header("X-Quota-Limit", strconv.Itoa(limit))
			c.Header("X-Quota-Remaining", "0")
			c.Header("Retry-After", strconv.Itoa(secondsUntilUTCMidnight()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "You've reached today's limit (" + strconv.Itoa(limit) + "). It resets at midnight UTC.",
				"code":  "quota_exceeded",
			})
			return
		}

		c.Set(quotaKey, quotaClaim{kind: kind, userID: user.ID, day: day})
		c.Header("X-Quota-Limit", strconv.Itoa(limit))
		c.Header("X-Quota-Remaining", strconv.Itoa(max(limit-used, 0)))
		c.Next()
	}
}

// RefundQuota gives back the use claimed by Quota for this request. It is safe to call when no
// quota was claimed, and refunds at most once per request.
func RefundQuota(c *gin.Context) {
	v, _ := c.Get(quotaKey)
	claim, ok := v.(quotaClaim)
	if !ok {
		return
	}
	c.Set(quotaKey, nil) // a second call finds nothing to refund

	query := refundSummarySQL
	if claim.kind == QuotaChats {
		query = refundChatSQL
	}
	if err := initializers.DB.Exec(query, claim.userID, claim.day).Error; err != nil {
		slog.Error("quota refund failed", "request_id", RequestIDFrom(c), "error", err)
	}
}

func secondsUntilUTCMidnight() int {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return int(next.Sub(now).Seconds()) + 1
}

// UsageToday returns how much of each daily allowance the user has used.
func UsageToday(userID uint) (summaries, chats int, err error) {
	row := initializers.DB.Raw(
		`SELECT COALESCE(SUM(summaries), 0), COALESCE(SUM(chats), 0) FROM daily_usage WHERE user_id = ? AND day = CURRENT_DATE`, userID).Row()
	err = row.Scan(&summaries, &chats)
	return
}
