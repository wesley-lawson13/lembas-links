package middleware

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

// setupAuthTestDB connects to the test database.
// Skips the test if TEST_DATABASE_URL is not set.
func setupAuthTestDB(t *testing.T) *sql.DB {
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

func newAuthTestRouter(store *models.URLStore, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuth(store, cfg))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func doAuthRequest(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAPIKeyAuth_MissingHeader sends a request with no Authorization header
// at all and confirms APIKeyAuth rejects it with 401 rather than treating an
// absent header as anonymous/allowed.
func TestAPIKeyAuth_MissingHeader(t *testing.T) {
	db := setupAuthTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30}
	router := newAuthTestRouter(store, cfg)

	w := doAuthRequest(router, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

// TestAPIKeyAuth_UnknownKey sends a well-formed but never-issued key and
// confirms APIKeyAuth rejects it with 401, proving the middleware actually
// validates against stored keys rather than accepting any bearer-shaped token.
func TestAPIKeyAuth_UnknownKey(t *testing.T) {
	db := setupAuthTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30}
	router := newAuthTestRouter(store, cfg)

	w := doAuthRequest(router, "Bearer not-a-real-key")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

// TestAPIKeyAuth_ExpiredKey inserts a key row with expires_at already in the
// past and confirms APIKeyAuth rejects it with 401 — a key existing in the
// table isn't sufficient on its own, it must also still be within its TTL.
func TestAPIKeyAuth_ExpiredKey(t *testing.T) {
	db := setupAuthTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30}
	router := newAuthTestRouter(store, cfg)

	rawKey := "auth-test-expired-key"
	hashedKey := models.HashKey(rawKey)
	t.Cleanup(func() { db.Exec("DELETE FROM api_keys WHERE key = $1", hashedKey) })

	_, err := db.Exec(
		"INSERT INTO api_keys (key, expires_at) VALUES ($1, $2)",
		hashedKey, time.Now().Add(-1*time.Hour),
	)
	if err != nil {
		t.Fatalf("failed to insert expired key: %v", err)
	}

	w := doAuthRequest(router, "Bearer "+rawKey)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

// TestAPIKeyAuth_ValidKey creates a real, unexpired key via store.CreateKey
// and confirms APIKeyAuth admits it with 200 — the happy-path counterpart to
// the three rejection cases above, proving the middleware isn't just failing
// closed on everything.
func TestAPIKeyAuth_ValidKey(t *testing.T) {
	db := setupAuthTestDB(t)
	store := models.NewURLStore(db)
	cfg := &config.Config{DefaultTTLDays: 30}
	router := newAuthTestRouter(store, cfg)

	rawKey, err := store.CreateKey(24 * time.Hour)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM api_keys WHERE key = $1", models.HashKey(rawKey)) })

	w := doAuthRequest(router, "Bearer "+rawKey)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}
