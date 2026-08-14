package handlers

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// setupTestDB connects to the test database.
// Skips the test if TEST_DATABASE_URL is not set.
func setupTestDB(t *testing.T) *sql.DB {
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
// both the stats and links handler tests that need an existing link to request.
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
