package handlers

import (
	"database/sql"
	"encoding/json"
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

// seedURL inserts a url row owned by hashedKey under a least-used quote slug,
// standing in for the removed store.GetSlug + store.CreateURL pair. Called from
// the handler tests that need an existing link to request. 
func seedURL(t *testing.T, db *sql.DB, hashedKey string, expiresAt time.Time) string {
	t.Helper()

	var slug string
	err := db.QueryRow(`
        SELECT q.slug
        FROM quotes q
        WHERE NOT EXISTS (SELECT 1 FROM urls u WHERE u.slug = q.slug)
        ORDER BY q.use_count, RANDOM()
        LIMIT 1
    `).Scan(&slug)
	if err != nil {
		t.Fatalf("failed to pick a test slug: %v", err)
	}

	_, err = db.Exec(`
        INSERT INTO urls (slug, original, api_key, expires_at)
        VALUES ($1, $2, $3, $4)
    `, slug, "https://example.com", hashedKey, expiresAt)
	if err != nil {
		t.Fatalf("failed to seed test url %q: %v", slug, err)
	}

	return slug
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

// TestGetStats_OwnershipMismatchReturns404 seeds a link owned by one API key
// and asserts GetStats returns 404 for a non-owning key before returning 200
// for the actual owner, mirroring the anti-enumeration behavior DeleteLink
// enforces for the same slug/key pairing.
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

	slug := seedURL(t, db, models.HashKey(rawKeyA), time.Now().Add(30*24*time.Hour))

	w := doStatsRequest(router, slug, "Bearer "+rawKeyB)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owning key, got %d", w.Code)
	}

	w = doStatsRequest(router, slug, "Bearer "+rawKeyA)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owning key, got %d", w.Code)
	}
}

// TestGetStats_OwnerSeesExpiredLink verifies that expiry stops redirects, not
// the owner's analytics: stats for an expired link return 200 for the owner
// while non-owners still get the anti-enumeration 404.
func TestGetStats_OwnerSeesExpiredLink(t *testing.T) {
	db := setupStatsTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30, RecentClicksLimit: 10}
	router := newStatsTestRouter(store, cfg)

	rawKeyA := "stats-test-expired-owner-key"
	rawKeyB := "stats-test-expired-other-key"
	t.Cleanup(func() {
		db.Exec("DELETE FROM clicks WHERE slug IN (SELECT slug FROM urls WHERE api_key = $1)", models.HashKey(rawKeyA))
		db.Exec("DELETE FROM urls WHERE api_key = $1", models.HashKey(rawKeyA))
	})

	slug := seedURL(t, db, models.HashKey(rawKeyA), time.Now().Add(-24*time.Hour))

	w := doStatsRequest(router, slug, "Bearer "+rawKeyA)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner of expired link, got %d", w.Code)
	}

	var body struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode stats response: %v", err)
	}
	if !body.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expected expires_at in the past, got %v", body.ExpiresAt)
	}

	w = doStatsRequest(router, slug, "Bearer "+rawKeyB)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owning key on expired link, got %d", w.Code)
	}
}

// TestGetStats_IncludesDailyClicksAndShortURL verifies the owner's stats
// response carries a 7-entry daily_clicks series and a short_url built from
// the configured base URL.
func TestGetStats_IncludesDailyClicksAndShortURL(t *testing.T) {
	db := setupStatsTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30, RecentClicksLimit: 10, BaseURL: "http://localhost:8080"}
	router := newStatsTestRouter(store, cfg)

	rawKey := "stats-test-daily-clicks-key"
	t.Cleanup(func() {
		db.Exec("DELETE FROM clicks WHERE slug IN (SELECT slug FROM urls WHERE api_key = $1)", models.HashKey(rawKey))
		db.Exec("DELETE FROM urls WHERE api_key = $1", models.HashKey(rawKey))
	})

	slug := seedURL(t, db, models.HashKey(rawKey), time.Now().Add(30*24*time.Hour))

	w := doStatsRequest(router, slug, "Bearer "+rawKey)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owning key, got %d", w.Code)
	}

	var body struct {
		ShortURL    string `json:"short_url"`
		DailyClicks []struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		} `json:"daily_clicks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode stats response: %v", err)
	}

	if len(body.DailyClicks) != 7 {
		t.Errorf("expected 7 daily_clicks entries, got %d", len(body.DailyClicks))
	}
	wantShortURL := cfg.BaseURL + "/" + slug
	if body.ShortURL == "" {
		t.Error("expected non-empty short_url in stats response")
	} else if body.ShortURL != wantShortURL {
		t.Errorf("expected short_url %q, got %q", wantShortURL, body.ShortURL)
	}
}

// TestRedirect_ExpiredLinkReturns410AndStaysActive verifies that redirecting
// an expired slug returns 410 without soft-deleting the row, so the link
// keeps appearing in the owner's list and stats.
func TestRedirect_ExpiredLinkReturns410AndStaysActive(t *testing.T) {
	db := setupStatsTestDB(t)
	redisClient := setupLinksTestRedis(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30, RecentClicksLimit: 10}

	gin.SetMode(gin.TestMode)
	lh := NewLinkHandler(store, redisClient, cfg)
	router := gin.New()
	router.GET("/:slug", lh.Redirect)

	rawKey := "stats-test-redirect-expired-key"
	t.Cleanup(func() {
		db.Exec("DELETE FROM clicks WHERE slug IN (SELECT slug FROM urls WHERE api_key = $1)", models.HashKey(rawKey))
		db.Exec("DELETE FROM urls WHERE api_key = $1", models.HashKey(rawKey))
	})

	slug := seedURL(t, db, models.HashKey(rawKey), time.Now().Add(-24*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/"+slug, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for expired link, got %d", w.Code)
	}

	url, err := store.GetURL(slug)
	if err != nil {
		t.Fatalf("GetURL failed: %v", err)
	}
	if !url.IsActive {
		t.Fatalf("expected expired link to stay active after redirect attempt")
	}

	urls, err := store.ListURLsByAPIKey(rawKey)
	if err != nil {
		t.Fatalf("ListURLsByAPIKey failed: %v", err)
	}
	found := false
	for _, u := range urls {
		if u.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected expired link %s to remain in owner's list", slug)
	}
}
