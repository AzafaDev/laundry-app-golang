package testutil

import (
	"laundry-app-with-golang/internal/app"
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// TestApp bundles the router with the DB handles fixtures need to insert
// and clean up rows directly.
type TestApp struct {
	Router  *gin.Engine
	Queries *db.Queries
	Pool    *pgxpool.Pool
}

// loadRepoRootEnv pre-loads the repo-root .env before config.Load() runs.
// `go test` sets the working directory to the package under test, so
// config.Load()'s own godotenv.Load() (which only checks cwd) never finds
// it; godotenv.Load() doesn't override already-set vars, so pre-loading
// here is transparent to config.Load()'s own call.
func loadRepoRootEnv() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	// this file lives at <repo root>/internal/testutil/testutil.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	_ = godotenv.Load(filepath.Join(repoRoot, ".env"))
}

// NewTestApp wires up a real router against the DATABASE_URL/.env configured
// for the test environment, and closes the pool when the test finishes.
func NewTestApp(t *testing.T) *TestApp {
	t.Helper()

	loadRepoRootEnv()
	cfg := config.Load()

	router, pool, err := app.New(cfg)
	if err != nil {
		t.Fatalf("failed to start test app: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	return &TestApp{
		Router:  router,
		Queries: db.New(pool),
		Pool:    pool,
	}
}
