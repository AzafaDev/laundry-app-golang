package testutil

import (
	"laundry-app-with-golang/internal/app"
	"laundry-app-with-golang/internal/config"
	"testing"

	"github.com/gin-gonic/gin"
)

// NewTestApp wires up a real router against the DB_URL/.env configured for
// the test environment, and closes the pool when the test finishes.
func NewTestApp(t *testing.T) *gin.Engine {
	t.Helper()

	cfg := config.Load()

	router, pool, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to start test app: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	return router
}
