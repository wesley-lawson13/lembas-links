package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wesley-lawson13/lembas-links/config"
	"github.com/wesley-lawson13/lembas-links/models"
)

// setupLinksTestRedis connects to the test Redis instance.
// Skips the test if TEST_REDIS_URL is not set.
func setupLinksTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	dsn := os.Getenv("TEST_REDIS_URL")
	if dsn == "" {
		t.Skip("TEST_REDIS_URL not set, skipping integration tests")
	}

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		t.Fatalf("failed to parse TEST_REDIS_URL: %v", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("failed to connect to test redis: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func newLinksTestRouter(store *models.URLStore, redisClient *redis.Client, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	lh := NewLinkHandler(store, redisClient, cfg)
	r := gin.New()
	r.DELETE("/links/:slug", lh.DeleteLink)
	return r
}

func doDeleteRequest(r *gin.Engine, slug, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/links/"+slug, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDeleteLink_OwnershipMismatchReturns404(t *testing.T) {
	db := setupStatsTestDB(t)
	redisClient := setupLinksTestRedis(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30, RecentClicksLimit: 10}
	router := newLinksTestRouter(store, redisClient, cfg)

	rawKeyA := "links-test-owner-key"
	rawKeyB := "links-test-other-key"
	t.Cleanup(func() {
		db.Exec("DELETE FROM urls WHERE api_key = $1", models.HashKey(rawKeyA))
	})

	slug, err := store.GetSlug()
	if err != nil {
		t.Fatalf("GetSlug failed: %v", err)
	}
	if err := store.CreateURL(slug, "https://example.com", models.HashKey(rawKeyA), time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("CreateURL failed: %v", err)
	}

	w := doDeleteRequest(router, slug, "Bearer "+rawKeyB)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owning key, got %d", w.Code)
	}

	url, err := store.GetURL(slug)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}
	if !url.IsActive {
		t.Fatalf("expected link to remain active after non-owner delete attempt")
	}

	w = doDeleteRequest(router, slug, "Bearer "+rawKeyA)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for owning key, got %d", w.Code)
	}
}
