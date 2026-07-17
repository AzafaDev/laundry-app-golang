package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"laundry-app-with-golang/internal/sse"
	"laundry-app-with-golang/internal/testutil"

	"github.com/gin-gonic/gin"
)

// syncRecorder is a minimal http.ResponseWriter safe for the concurrent
// access this test needs: Stream() writes to it from a goroutine while the
// test polls its contents from another. httptest.ResponseRecorder's
// bytes.Buffer isn't safe for that combination.
type syncRecorder struct {
	mu     sync.Mutex
	header http.Header
	body   strings.Builder
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{header: make(http.Header)}
}

func (r *syncRecorder) Header() http.Header { return r.header }

func (r *syncRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(b)
}

func (r *syncRecorder) WriteHeader(statusCode int) {}

func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func TestStream_DeliversBroadcastEvent(t *testing.T) {
	app := testutil.NewTestApp(t)
	customer := app.CreateTestCustomer(t)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)
	accessToken := testutil.CookieValue(cookies, "access_token")
	if accessToken == "" {
		t.Fatal("expected access_token cookie from login")
	}

	handler := sse.NewHandler(app.Queries, app.Cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := newSyncRecorder()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(writer)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: accessToken})
	c.Request = req

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.Stream(c)
	}()

	channel := "user:" + customer.ID.String()
	const wantEvent = "order:status-updated"
	deadline := time.Now().Add(5 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		sse.Default.Broadcast(channel, wantEvent, map[string]string{"orderID": "test-order"})

		body := writer.String()
		if strings.Contains(body, "event: "+wantEvent) && strings.Contains(body, "data: ") {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	if !found {
		t.Fatalf("expected to see %q event in stream body, got: %s", wantEvent, writer.String())
	}
}
