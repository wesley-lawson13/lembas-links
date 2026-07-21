package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/models"
)

// setupStatsTestDB connects to the test database.
// Skips the test if TEST_DATABASE_URL is not set.
func setupStatsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newStatsTestRouter(store *models.URLStore, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	lh := NewLinkHandler(store, nil, cfg)
	r := gin.New()
	r.GET("/links/:slug/stats", lh.GetStats)
	return r
}

func doStatsRequest(r *gin.Engine, slug, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/links/"+slug+"/stats", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetStats_OwnershipMismatchReturns404(t *testing.T) {
	db := setupStatsTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30, RecentClicksLimit: 10}
	router := newStatsTestRouter(store, cfg)

	rawKeyA := "stats-test-owner-key"
	rawKeyB := "stats-test-other-key"
	t.Cleanup(func() {
		db.Exec("DELETE FROM clicks WHERE slug IN (SELECT slug FROM urls WHERE api_key = $1)", models.HashKey(rawKeyA))
		db.Exec("DELETE FROM urls WHERE api_key = $1", models.HashKey(rawKeyA))
	})

	slug, err := store.GetSlug()
	if err != nil {
		t.Fatalf("GetSlug failed: %v", err)
	}
	if err := store.CreateURL(slug, "https://example.com", models.HashKey(rawKeyA), time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("CreateURL failed: %v", err)
	}

	w := doStatsRequest(router, slug, "Bearer "+rawKeyB)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owning key, got %d", w.Code)
	}

	w = doStatsRequest(router, slug, "Bearer "+rawKeyA)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owning key, got %d", w.Code)
	}
}
