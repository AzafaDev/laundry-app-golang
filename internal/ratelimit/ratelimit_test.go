package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func newTestRouter(m gin.HandlerFunc, status int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(m)
	router.GET("/", func(c *gin.Context) {
		c.Status(status)
	})
	return router
}

func doRequest(router *gin.Engine) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)
	return w.Code
}

func TestMiddleware_AllowsUnderBudgetRejectsOverBudget(t *testing.T) {
	const burst = 5
	// Slow refill so no extra tokens trickle in during the test.
	limiter := NewLimiter(rate.Every(time.Hour), burst)
	router := newTestRouter(Middleware(limiter, false), http.StatusOK)

	for i := 0; i < burst; i++ {
		if code := doRequest(router); code != http.StatusOK {
			t.Fatalf("request %d: got status %d, want %d", i+1, code, http.StatusOK)
		}
	}

	if code := doRequest(router); code != http.StatusTooManyRequests {
		t.Errorf("request %d (over budget): got status %d, want %d", burst+1, code, http.StatusTooManyRequests)
	}
}

func TestMiddleware_SkipSuccessfulRefundsToken(t *testing.T) {
	const burst = 3
	limiter := NewLimiter(rate.Every(time.Hour), burst)
	router := newTestRouter(Middleware(limiter, true), http.StatusOK)

	// Far more successful requests than the burst size — each one's token
	// should be refunded, so none of these should ever trip the limiter.
	for i := 0; i < burst*10; i++ {
		if code := doRequest(router); code != http.StatusOK {
			t.Fatalf("successful request %d: got status %d, want %d (skipSuccessful should refund the token)", i+1, code, http.StatusOK)
		}
	}
}

func TestMiddleware_SkipSuccessfulStillCountsFailures(t *testing.T) {
	const burst = 3
	limiter := NewLimiter(rate.Every(time.Hour), burst)
	router := newTestRouter(Middleware(limiter, true), http.StatusUnauthorized)

	for i := 0; i < burst; i++ {
		if code := doRequest(router); code != http.StatusUnauthorized {
			t.Fatalf("failed request %d: got status %d, want %d", i+1, code, http.StatusUnauthorized)
		}
	}

	if code := doRequest(router); code != http.StatusTooManyRequests {
		t.Errorf("request %d (over budget after failures): got status %d, want %d", burst+1, code, http.StatusTooManyRequests)
	}
}
