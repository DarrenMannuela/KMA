package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// keyLimiter is a per-key (user or IP) token bucket — the same pattern
// KMA-Auth's rate limiter uses, duplicated here rather than shared since
// these are separate Go modules with no common internal package today.
type keyLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

func newKeyLimiter(r rate.Limit, burst int) *keyLimiter {
	l := &keyLimiter{limiters: make(map[string]*rate.Limiter), r: r, burst: burst}
	go l.cleanupLoop()
	return l
}

func (l *keyLimiter) get(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.limiters[key]; ok {
		return lim
	}
	lim := rate.NewLimiter(l.r, l.burst)
	l.limiters[key] = lim
	return lim
}

// cleanupLoop prevents unbounded memory growth from one-off keys. Not
// perfectly precise (a limiter can be evicted mid-life and restart
// fresh) — acceptable for a lightweight in-process limiter serving a
// single small team.
func (l *keyLimiter) cleanupLoop() {
	for {
		time.Sleep(10 * time.Minute)
		l.mu.Lock()
		for key, lim := range l.limiters {
			if lim.Tokens() == float64(l.burst) { // back at full burst == idle
				delete(l.limiters, key)
			}
		}
		l.mu.Unlock()
	}
}

// batchLimiter: 10 requests/minute steady state, burst of 10 — generous
// enough for someone actively working through a sheet and saving
// repeatedly, tight enough to cap a buggy frontend retry loop or a
// compromised session from doing rapid bulk damage through the
// batch endpoints (each call can touch up to ~2000 rows, deletes
// included).
var batchLimiter = newKeyLimiter(rate.Every(6*time.Second), 10)

// RateLimitBatch throttles the bulk-edit endpoints (finance/bukti-kas/
// catalogue batch) per logged-in user rather than per IP — the actual
// risk here is one account doing too much too fast, not one network
// address, and every route this is attached to already sits behind
// RequireAuth so a user_id is always present in context by the time
// this runs.
func RateLimitBatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if uid, ok := c.Get("user_id"); ok {
			key = fmt.Sprintf("user:%v", uid)
		}
		lim := batchLimiter.get(key)
		if !lim.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many bulk changes too quickly, please wait and try again",
			})
			return
		}
		c.Next()
	}
}
