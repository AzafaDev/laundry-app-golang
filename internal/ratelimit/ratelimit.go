package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"laundry-app-with-golang/internal/apperr"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const cleanupInterval = time.Minute
const staleAfter = 30 * time.Minute

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is a per-key (typically per-IP) token bucket limiter. It runs a
// background goroutine to evict idle keys so memory doesn't grow unbounded
// for a long-running, single-instance process (see ticket #10).
type Limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	r        rate.Limit
	burst    int
}

func NewLimiter(r rate.Limit, burst int) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		r:        r,
		burst:    burst,
	}
	go l.cleanupLoop()
	return l
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for key, v := range l.visitors {
			if time.Since(v.lastSeen) > staleAfter {
				delete(l.visitors, key)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) get(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.r, l.burst)}
		l.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// Middleware enforces l against c.ClientIP(). If skipSuccessful is true, a
// request whose handler finishes with a status below 400 has its token
// refunded, so only failed attempts (e.g. failed logins) count against the
// budget.
func Middleware(l *Limiter, skipSuccessful bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := l.get(c.ClientIP())

		// Pin one timestamp and reuse it for Reserve/Delay/Cancel below.
		// Reservation.CancelAt(t) is a no-op once t is after the
		// reservation's timeToAct, which for a zero-delay reservation is
		// "immediately" — even the few nanoseconds between Reserve() and a
		// later Cancel() (after the handler runs) are enough to make a
		// fresh time.Now()-based Cancel() silently fail to refund. Reusing
		// the exact Reserve time keeps the cancel valid.
		now := time.Now()
		res := limiter.ReserveN(now, 1)
		if !res.OK() || res.DelayFrom(now) > 0 {
			res.CancelAt(now)
			apperr.AbortWithError(c, http.StatusTooManyRequests, "rate_limited")
			return
		}

		c.Next()

		if skipSuccessful && c.Writer.Status() < 400 {
			res.CancelAt(now)
		}
	}
}
