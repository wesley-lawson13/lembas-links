package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// newLinksTestRouter wires a real LinkHandler (backed by the given store,
// Redis client, and config) to a bare DELETE /links/:slug route.
// Called from TestDeleteLink_OwnershipMismatchReturns404.
func newLinksTestRouter(store *models.URLStore, redisClient *redis.Client, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	lh := NewLinkHandler(store, redisClient, cfg)
	r := gin.New()
	r.DELETE("/links/:slug", lh.DeleteLink)
	return r
}

// doDeleteRequest sends a DELETE /links/:slug request through the router,
// optionally setting authHeader as the Authorization header, and returns the
// recorded response. Called from TestDeleteLink_OwnershipMismatchReturns404.
func doDeleteRequest(r *gin.Engine, slug, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/links/"+slug, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestValidateTargetURL exercises validateTargetURL directly against allowed
// (http/https, case-insensitive scheme) and disallowed inputs (javascript:,
// data:, file:, mailto:, scheme-less, host-less, over-length) — ensuring the
// safeguard against CreateLink being used as an open redirector works correctly.
func TestValidateTargetURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"plain https", "https://example.com/some/path", false},
		{"plain http", "http://example.com", false},
		{"uppercase scheme", "HTTPS://example.com", false},
		{"javascript scheme", "javascript:alert(1)", true},
		{"data scheme", "data:text/html,<script>alert(1)</script>", true},
		{"file scheme", "file:///etc/passwd", true},
		{"mailto scheme", "mailto:someone@example.com", true},
		{"scheme-less", "example.com/path", true},
		{"host-less http", "http://", true},
		{"over length", "https://example.com/" + strings.Repeat("a", maxURLLength), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTargetURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.url, err)
			}
		})
	}
}

// TestCreateLink_RejectsInvalidURL confirms the handler returns 400 for a
// disallowed URL before it ever reaches the store — so a nil store is safe
// here and proves validation runs first.
func TestCreateLink_RejectsInvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{DefaultTTLDays: 30, BaseURL: "http://localhost:8080"}
	lh := NewLinkHandler(nil, nil, cfg)
	r := gin.New()
	r.POST("/links", lh.CreateLink)

	req := httptest.NewRequest(http.MethodPost, "/links", strings.NewReader(`{"url":"javascript:alert(1)"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for javascript: url, got %d", w.Code)
	}
}

// TestDeleteLink_OwnershipMismatchReturns404 seeds a link owned by one API
// key, then asserts DeleteLink returns 404 (and leaves the link active) for
// a non-owning key before returning 204 for the actual owner — verifying the
// handler never leaks slug existence to a caller who doesn't own it.
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

	slug := seedURL(t, db, models.HashKey(rawKeyA), time.Now().Add(30*24*time.Hour))

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
